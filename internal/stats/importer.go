package stats

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"
)

// exportMessage is the minimal fields we need from the Telegram JSON export.
type exportMessage struct {
	ID           int64  `json:"id"`
	Type         string `json:"type"`
	DateUnixtime string `json:"date_unixtime"`
	From         string `json:"from"`
	FromID       string `json:"from_id"`
}

// exportFile is the top-level structure of a Telegram JSON export.
type exportFile struct {
	ID       int64           `json:"id"`
	Messages []exportMessage `json:"messages"`
}

// ImportResult summarises the outcome of an import run.
type ImportResult struct {
	MessagesProcessed int
	BucketsWritten    int
	UsersUpdated      int
	CutoffUTC         time.Time
}

// ImportFromExport reads a Telegram JSON export from r, derives the cutoff
// (last complete hour in Europe/Madrid relative to the latest message in the
// export), and writes aggregated hourly stats to the database.
//
// For the affected bucket range the existing rows are overwritten (deleted +
// inserted), then user totals are recomputed from the hourly table.
func (s *Service) ImportFromExport(ctx context.Context, chatID int64, r io.Reader) (*ImportResult, error) {
	raw, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("stats import: read input: %w", err)
	}

	var export exportFile
	if err := json.Unmarshal(raw, &export); err != nil {
		return nil, fmt.Errorf("stats import: parse JSON: %w", err)
	}

	// If chatID is not explicitly provided, derive it from the export.
	if chatID == 0 {
		chatID = export.ID
	}

	// --- single pass: aggregate counts per (userID, bucket), track max time -
	type userBucket struct {
		userName string
		count    int64
	}
	buckets := make(map[time.Time]map[int64]*userBucket)

	var maxTime time.Time
	processed := 0

	for i := range export.Messages {
		m := &export.Messages[i]
		if m.Type != "message" {
			continue
		}
		t, err := parseExportTime(m)
		if err != nil {
			continue
		}
		if t.After(maxTime) {
			maxTime = t
		}

		userID, ok := parseFromID(m.FromID)
		if !ok || m.From == "" {
			continue
		}

		bucketTS := t.UTC().Truncate(time.Hour)
		users := buckets[bucketTS]
		if users == nil {
			users = make(map[int64]*userBucket)
			buckets[bucketTS] = users
		}
		ub := users[userID]
		if ub == nil {
			ub = &userBucket{userName: m.From}
			users[userID] = ub
		}
		ub.count++
		ub.userName = m.From
		processed++
	}

	if maxTime.IsZero() {
		return &ImportResult{}, nil
	}

	cutoffUTC := maxTime.Truncate(time.Hour)
	// remove the one incomplete hour (the hour containing the latest message)
	delete(buckets, cutoffUTC.Add(time.Hour))

	if len(buckets) == 0 {
		return &ImportResult{MessagesProcessed: processed, CutoffUTC: cutoffUTC}, nil
	}

	// --- compute affected bucket range -------------------------------------
	var minBucket, maxBucket time.Time
	for bucketTS := range buckets {
		if minBucket.IsZero() || bucketTS.Before(minBucket) {
			minBucket = bucketTS
		}
		if maxBucket.IsZero() || bucketTS.After(maxBucket) {
			maxBucket = bucketTS
		}
	}

	// --- database writes (single transaction) ------------------------------
	tx := s.db.WithContext(ctx).Begin()
	if tx.Error != nil {
		return nil, fmt.Errorf("stats import: begin tx: %w", tx.Error)
	}
	if err := lockChatTx(tx, chatID); err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("stats import: lock chat: %w", err)
	}

	// Delete existing hourly rows in the affected range for this chat.
	if err := tx.Exec(
		`DELETE FROM user_message_hourly WHERE chat_id = ? AND bucket_ts >= ? AND bucket_ts <= ?`,
		chatID, minBucket, maxBucket,
	).Error; err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("stats import: delete hourly: %w", err)
	}

	// Insert aggregated buckets.
	for bucketTS, users := range buckets {
		for userID, ub := range users {
			if err := tx.Exec(
				`INSERT INTO user_message_hourly (chat_id, user_id, user_name, bucket_ts, message_count, updated_at)
				 VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP)`,
				chatID, userID, ub.userName, bucketTS, ub.count,
			).Error; err != nil {
				tx.Rollback()
				return nil, fmt.Errorf("stats import: insert hourly: %w", err)
			}
		}
	}

	// Recompute user_message_stats totals for all users in this chat whose
	// buckets overlap the affected range.
	recomputeSQL := `
INSERT INTO user_message_stats (chat_id, user_id, user_name, last_message_at, total_messages, updated_at)
SELECT
    h.chat_id,
    h.user_id,
    (SELECT user_name FROM user_message_hourly
     WHERE chat_id = h.chat_id AND user_id = h.user_id
     ORDER BY bucket_ts DESC LIMIT 1) AS user_name,
    (SELECT bucket_ts + INTERVAL '1 hour'
     FROM user_message_hourly
     WHERE chat_id = h.chat_id AND user_id = h.user_id
     ORDER BY bucket_ts DESC LIMIT 1) AS last_message_at,
    SUM(h.message_count) AS total_messages,
    CURRENT_TIMESTAMP
FROM user_message_hourly h
WHERE h.chat_id = ?
  AND h.user_id IN (
      SELECT DISTINCT user_id FROM user_message_hourly
      WHERE chat_id = ? AND bucket_ts >= ? AND bucket_ts <= ?
  )
GROUP BY h.chat_id, h.user_id
ON CONFLICT (chat_id, user_id) DO UPDATE
SET total_messages  = EXCLUDED.total_messages,
    last_message_at = EXCLUDED.last_message_at,
    user_name       = EXCLUDED.user_name,
    updated_at      = CURRENT_TIMESTAMP;
`
	result := tx.Exec(recomputeSQL, chatID, chatID, minBucket, maxBucket)
	if result.Error != nil {
		tx.Rollback()
		return nil, fmt.Errorf("stats import: recompute totals: %w", result.Error)
	}
	usersUpdated := int(result.RowsAffected)

	if err := tx.Commit().Error; err != nil {
		return nil, fmt.Errorf("stats import: commit: %w", err)
	}

	return &ImportResult{
		MessagesProcessed: processed,
		BucketsWritten:    len(buckets),
		UsersUpdated:      usersUpdated,
		CutoffUTC:         cutoffUTC,
	}, nil
}

// parseExportTime converts an exportMessage timestamp to UTC.
// It uses date_unixtime as it comes in UTC always.
func parseExportTime(m *exportMessage) (time.Time, error) {
	if m.DateUnixtime != "" {
		epoch, err := strconv.ParseInt(strings.TrimSpace(m.DateUnixtime), 10, 64)
		if err == nil {
			return time.Unix(epoch, 0).UTC(), nil
		}
	}

	return time.Time{}, fmt.Errorf("no valid date_unixtime in message id=%d", m.ID)
}

// parseFromID converts a Telegram export from_id string (e.g. "user502546420"
// or "channel1549724404") to a numeric int64 user/chat ID.
func parseFromID(fromID string) (int64, bool) {
	for _, prefix := range []string{"user", "channel"} {
		if strings.HasPrefix(fromID, prefix) {
			n, err := strconv.ParseInt(fromID[len(prefix):], 10, 64)
			if err == nil {
				return n, true
			}
		}
	}
	return 0, false
}
