package stats

import (
	"context"
	"time"

	"gorm.io/gorm"
)

const statsAdvisoryLockNamespace int64 = 0x73746174 // "stat"

const userStatsUpsertSQL = `
INSERT INTO user_message_stats (
    chat_id,
    user_id,
    user_name,
    last_message_at,
    total_messages,
    updated_at
) VALUES (
    ?, ?, ?, ?, 1, CURRENT_TIMESTAMP
)
ON CONFLICT (chat_id, user_id) DO UPDATE
SET total_messages = user_message_stats.total_messages + 1,
    last_message_at = GREATEST(user_message_stats.last_message_at, EXCLUDED.last_message_at),
    user_name = EXCLUDED.user_name,
    updated_at = CURRENT_TIMESTAMP;
`

const userHourlyUpsertSQL = `
INSERT INTO user_message_hourly (
    chat_id,
    user_id,
    user_name,
    bucket_ts,
    message_count,
    updated_at
) VALUES (
    ?, ?, ?, ?, 1, CURRENT_TIMESTAMP
)
ON CONFLICT (chat_id, user_id, bucket_ts) DO UPDATE
SET message_count = user_message_hourly.message_count + 1,
    user_name = EXCLUDED.user_name,
    updated_at = CURRENT_TIMESTAMP;
`

// Service provides user message tracking operations.
type Service struct {
	db *gorm.DB
}

// NewService creates a new stats service.
func NewService(db *gorm.DB) *Service {
	return &Service{db: db}
}

// RecordMessage updates user totals and hourly buckets for a message.
func (s *Service) RecordMessage(ctx context.Context, chatID, userID int64, userName string, messageTime time.Time) error {
	bucketTime := bucketTime(messageTime)

	tx := s.db.WithContext(ctx).Begin()
	if tx.Error != nil {
		return tx.Error
	}
	if err := lockChatTx(tx, chatID); err != nil {
		tx.Rollback()
		return err
	}

	if err := tx.Exec(userStatsUpsertSQL, chatID, userID, userName, messageTime).Error; err != nil {
		tx.Rollback()
		return err
	}

	if err := tx.Exec(userHourlyUpsertSQL, chatID, userID, userName, bucketTime).Error; err != nil {
		tx.Rollback()
		return err
	}

	return tx.Commit().Error
}

func lockChatTx(tx *gorm.DB, chatID int64) error {
	if tx == nil {
		return nil
	}
	if tx.Dialector == nil || tx.Dialector.Name() != "postgres" {
		return nil
	}

	// Serialize updates per chat so imports and live updates can't clobber each
	// other (e.g. importer recompute overwriting concurrent RecordMessage totals).
	lockKey := chatID ^ (statsAdvisoryLockNamespace << 32)
	return tx.Exec(`SELECT pg_advisory_xact_lock(?)`, lockKey).Error
}

// UserStats represents summary stats for a user.
type UserStats struct {
	UserID        int64     `gorm:"column:user_id"`
	UserName      string    `gorm:"column:user_name"`
	LastMessageAt time.Time `gorm:"column:last_message_at"`
	TotalMessages int64     `gorm:"column:total_messages"`
}

// DailyCount represents message counts aggregated by day.
type DailyCount struct {
	Day          time.Time `gorm:"column:day"`
	MessageCount int64     `gorm:"column:message_count"`
}

// GetUserStatsByName retrieves the latest stats for a username within a chat.
func (s *Service) GetUserStatsByName(ctx context.Context, chatID int64, userName string) (*UserStats, error) {
	if userName == "" {
		return nil, nil
	}

	var stats UserStats
	result := s.db.WithContext(ctx).
		Raw(`
SELECT user_id, user_name, last_message_at, total_messages
FROM user_message_stats
WHERE chat_id = ? AND lower(user_name) = lower(?)
ORDER BY last_message_at DESC
LIMIT 1
`, chatID, userName).
		Scan(&stats)

	if result.Error != nil {
		return nil, result.Error
	}
	if result.RowsAffected == 0 {
		return nil, nil
	}

	return &stats, nil
}

// GetUserDailyCounts aggregates message counts by day for a user.
func (s *Service) GetUserDailyCounts(ctx context.Context, chatID, userID int64, start, end time.Time) ([]DailyCount, error) {
	var counts []DailyCount
	result := s.db.WithContext(ctx).
		Raw(`
SELECT date_trunc('day', bucket_ts) AS day,
       SUM(message_count) AS message_count
FROM user_message_hourly
WHERE chat_id = ?
  AND user_id = ?
  AND bucket_ts >= ?
  AND bucket_ts < ?
GROUP BY day
ORDER BY day ASC
`, chatID, userID, start, end).
		Scan(&counts)

	if result.Error != nil {
		return nil, result.Error
	}

	return counts, nil
}

// TopUser represents a user's ranking in a top-N query.
type TopUser struct {
	UserName     string `gorm:"column:user_name"`
	MessageCount int64  `gorm:"column:message_count"`
}

// GetTopUsersTotal returns the top N users by all-time message count in a chat.
func (s *Service) GetTopUsersTotal(ctx context.Context, chatID int64, limit int) ([]TopUser, error) {
	var users []TopUser
	result := s.db.WithContext(ctx).
		Raw(`
SELECT user_name, total_messages AS message_count
FROM user_message_stats
WHERE chat_id = ?
ORDER BY total_messages DESC
LIMIT ?
`, chatID, limit).
		Scan(&users)

	if result.Error != nil {
		return nil, result.Error
	}
	return users, nil
}

// GetTopUsersSince returns the top N users by message count since a given time.
func (s *Service) GetTopUsersSince(ctx context.Context, chatID int64, since time.Time, limit int) ([]TopUser, error) {
	var users []TopUser
	result := s.db.WithContext(ctx).
		Raw(`
SELECT (SELECT h2.user_name
        FROM user_message_hourly h2
        WHERE h2.chat_id = h.chat_id AND h2.user_id = h.user_id
        ORDER BY h2.bucket_ts DESC
        LIMIT 1) AS user_name,
       SUM(message_count) AS message_count
FROM user_message_hourly h
WHERE h.chat_id = ?
  AND h.bucket_ts >= ?
GROUP BY h.chat_id, h.user_id
ORDER BY message_count DESC
LIMIT ?
`, chatID, since, limit).
		Scan(&users)

	if result.Error != nil {
		return nil, result.Error
	}
	return users, nil
}

func bucketTime(t time.Time) time.Time {
	return t.UTC().Truncate(time.Hour)
}
