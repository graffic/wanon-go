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

// spainLocation is Europe/Madrid (GMT+1 standard / GMT+2 CEST).
var spainLocation *time.Location

func init() {
	loc, err := time.LoadLocation("Europe/Madrid")
	if err != nil {
		panic(fmt.Sprintf("stats: failed to load Europe/Madrid timezone: %v", err))
	}
	spainLocation = loc
}

// exportMessage is the minimal fields we need from the Telegram JSON export.
type exportMessage struct {
	ID           int64  `json:"id"`
	Type         string `json:"type"`
	Date         string `json:"date"`
	DateUnixtime string `json:"date_unixtime"`
	From         string `json:"from"`
	FromID       string `json:"from_id"`
}

// exportFile is the top-level structure of a Telegram JSON export.
type exportFile struct {
	ID       int64           `json:"id"`
	Messages []exportMessage `json:"messages"`
}

// hourlyKey identifies a unique (chatID, userID, bucketTS) combination.
type hourlyKey struct {
	chatID   int64
	userID   int64
	bucketTS time.Time
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

	// --- pass 1: find max message timestamp (type==message only) -----------
	var maxTime time.Time
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
	}

	if maxTime.IsZero() {
		return &ImportResult{}, nil
	}

	cutoffUTC := exportCutoff(maxTime)

	// --- pass 2: aggregate counts per (chatID, userID, bucket) -------------
	type bucketEntry struct {
		userName string
		count    int64
	}
	buckets := make(map[hourlyKey]*bucketEntry)

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
		// Only include messages whose hourly bucket is within the last complete hour.
		// We compare bucket timestamps so that any message within the cutoff hour
		// is included, regardless of its exact minute/second.
		if t.UTC().Truncate(time.Hour).After(cutoffUTC) {
			continue
		}

		userID, ok := parseFromID(m.FromID)
		if !ok || m.From == "" {
			continue
		}

		key := hourlyKey{
			chatID:   chatID,
			userID:   userID,
			bucketTS: t.UTC().Truncate(time.Hour),
		}
		e := buckets[key]
		if e == nil {
			e = &bucketEntry{userName: m.From}
			buckets[key] = e
		}
		e.count++
		// keep the most recent user_name seen for that bucket
		e.userName = m.From
		processed++
	}

	if len(buckets) == 0 {
		return &ImportResult{MessagesProcessed: processed, CutoffUTC: cutoffUTC}, nil
	}

	// --- compute affected bucket range -------------------------------------
	var minBucket, maxBucket time.Time
	for k := range buckets {
		if minBucket.IsZero() || k.bucketTS.Before(minBucket) {
			minBucket = k.bucketTS
		}
		if maxBucket.IsZero() || k.bucketTS.After(maxBucket) {
			maxBucket = k.bucketTS
		}
	}

	// --- database writes (single transaction) ------------------------------
	tx := s.db.WithContext(ctx).Begin()
	if tx.Error != nil {
		return nil, fmt.Errorf("stats import: begin tx: %w", tx.Error)
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
	for k, e := range buckets {
		if err := tx.Exec(
			`INSERT INTO user_message_hourly (chat_id, user_id, user_name, bucket_ts, message_count, updated_at)
			 VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP)`,
			k.chatID, k.userID, e.userName, k.bucketTS, e.count,
		).Error; err != nil {
			tx.Rollback()
			return nil, fmt.Errorf("stats import: insert hourly: %w", err)
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

// exportCutoff returns the last complete hour boundary (UTC) based on the
// maximum message timestamp in the export, using Europe/Madrid as the
// reference timezone.
//
// Example: if maxTime is 2022-08-20 12:45 (Madrid), the last complete hour
// started at 12:00 Madrid, so cutoff = 12:59:59.999… Madrid = that converted
// to UTC.  We represent the cutoff as the end of that hour bucket, i.e.
// bucket_start_UTC + 1h - 1ns (anything ≤ cutoff is included).
// In practice we compare message times with ≤ cutoff, and since we truncate
// to hour anyway, returning the truncated-to-hour UTC time is sufficient:
// messages in the same bucket (same truncated hour) are all ≤ their bucket.
//
// Concretely: cutoff = Truncate(maxTime in Madrid, hour) converted back to UTC.
func exportCutoff(maxTime time.Time) time.Time {
	madrid := maxTime.In(spainLocation)
	lastCompleteHour := madrid.Truncate(time.Hour)
	return lastCompleteHour.UTC()
}

// parseExportTime converts an exportMessage timestamp to UTC.
// It prefers date_unixtime (epoch string) to avoid any ambiguity.
// Falls back to parsing date as Europe/Madrid local time.
func parseExportTime(m *exportMessage) (time.Time, error) {
	if m.DateUnixtime != "" {
		epoch, err := strconv.ParseInt(strings.TrimSpace(m.DateUnixtime), 10, 64)
		if err == nil {
			return time.Unix(epoch, 0).UTC(), nil
		}
	}
	if m.Date != "" {
		t, err := time.ParseInLocation("2006-01-02T15:04:05", m.Date, spainLocation)
		if err == nil {
			return t.UTC(), nil
		}
	}
	return time.Time{}, fmt.Errorf("no valid timestamp in message id=%d", m.ID)
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
