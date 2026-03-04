package stats

import (
	"context"
	"time"

	"gorm.io/gorm"
)

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

func bucketTime(t time.Time) time.Time {
	return t.UTC().Truncate(time.Hour)
}
