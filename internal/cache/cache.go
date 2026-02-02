package cache

import (
	"context"
	"encoding/json"
	"time"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// CacheEntry represents a cached Telegram message
type CacheEntry struct {
	ID        uint           `gorm:"primarykey"`
	ChatID    int64          `gorm:"index;not null"`
	MessageID int64          `gorm:"index;not null"`
	ReplyID   *int64         `gorm:"index"`
	Date      int64          `gorm:"index;not null"`
	Message   datatypes.JSON `gorm:"type:jsonb;not null"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

// TableName specifies the table name for CacheEntry
func (CacheEntry) TableName() string {
	return "cache_entries"
}

// Service provides cache operations
type Service struct {
	db *gorm.DB
}

// NewService creates a new cache service
func NewService(db *gorm.DB) *Service {
	return &Service{db: db}
}

// Message represents a Telegram message for caching
type Message struct {
	MessageID int64           `json:"message_id"`
	Chat      Chat            `json:"chat"`
	Date      int64           `json:"date"`
	Text      string          `json:"text,omitempty"`
	From      *User           `json:"from,omitempty"`
	ReplyTo   *Message        `json:"reply_to_message,omitempty"`
	Raw       json.RawMessage `json:"-"`
}

// Chat represents a Telegram chat
type Chat struct {
	ID   int64  `json:"id"`
	Type string `json:"type"`
}

// User represents a Telegram user
type User struct {
	ID        int64  `json:"id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name,omitempty"`
	Username  string `json:"username,omitempty"`
}

// Add adds or updates a message in the cache
func (s *Service) Add(ctx context.Context, msg *Message) error {
	entry := &CacheEntry{
		ChatID:    msg.Chat.ID,
		MessageID: msg.MessageID,
		Date:      msg.Date,
	}

	if msg.ReplyTo != nil {
		entry.ReplyID = &msg.ReplyTo.MessageID
	}

	messageJSON, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	entry.Message = datatypes.JSON(messageJSON)

	// Use upsert to handle conflicts
	return s.db.WithContext(ctx).
		Where("chat_id = ? AND message_id = ?", entry.ChatID, entry.MessageID).
		Assign(entry).
		FirstOrCreate(entry).Error
}

// Edit updates a cached message with edited content
func (s *Service) Edit(ctx context.Context, msg *Message) error {
	var entry CacheEntry
	result := s.db.WithContext(ctx).
		Where("chat_id = ? AND message_id = ?", msg.Chat.ID, msg.MessageID).
		First(&entry)

	if result.Error == gorm.ErrRecordNotFound {
		// Message not in cache, nothing to update
		return nil
	}
	if result.Error != nil {
		return result.Error
	}

	// Update the message JSON
	messageJSON, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	return s.db.WithContext(ctx).
		Model(&entry).
		Update("message", datatypes.JSON(messageJSON)).Error
}

// Get retrieves a cached message by chat ID and message ID
func (s *Service) Get(ctx context.Context, chatID, messageID int64) (*CacheEntry, error) {
	var entry CacheEntry
	err := s.db.WithContext(ctx).
		Where("chat_id = ? AND message_id = ?", chatID, messageID).
		First(&entry).Error
	if err != nil {
		return nil, err
	}
	return &entry, nil
}

// GetByReply retrieves cached messages that reply to a specific message
func (s *Service) GetByReply(ctx context.Context, chatID, replyID int64) ([]CacheEntry, error) {
	var entries []CacheEntry
	err := s.db.WithContext(ctx).
		Where("chat_id = ? AND reply_id = ?", chatID, replyID).
		Order("date ASC").
		Find(&entries).Error
	return entries, err
}

// Clean removes cache entries older than the specified duration
func (s *Service) Clean(ctx context.Context, keepDuration time.Duration) error {
	cutoff := time.Now().Add(-keepDuration).Unix()
	return s.db.WithContext(ctx).
		Where("date < ?", cutoff).
		Delete(&CacheEntry{}).Error
}

// GetChain retrieves a chain of messages starting from a given message ID
// It follows reply chains recursively
func (s *Service) GetChain(ctx context.Context, chatID, messageID int64) ([]CacheEntry, error) {
	var entries []CacheEntry
	currentID := messageID
	seen := make(map[int64]bool)

	for {
		if seen[currentID] {
			// Prevent infinite loops
			break
		}
		seen[currentID] = true

		entry, err := s.Get(ctx, chatID, currentID)
		if err != nil {
			if err == gorm.ErrRecordNotFound {
				break
			}
			return nil, err
		}

		entries = append([]CacheEntry{*entry}, entries...)

		if entry.ReplyID == nil {
			break
		}
		currentID = *entry.ReplyID
	}

	return entries, nil
}
