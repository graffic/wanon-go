package quotes

import (
	"context"
	"encoding/json"
	"fmt"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// CacheEntry represents a cached message for building quotes
type CacheEntry struct {
	ID        uint           `gorm:"primaryKey"`
	ChatID    int64          `gorm:"index;not null"`
	MessageID int64          `gorm:"index;not null"`
	ReplyID   *int64         // Pointer to allow NULL
	Date      int64          `gorm:"not null"`
	Message   datatypes.JSON `gorm:"type:jsonb;not null"`
}

// TableName specifies the table name for CacheEntry
func (CacheEntry) TableName() string {
	return "cache_entry"
}

// Builder builds quote threads from cache entries by following reply chains
type Builder struct {
	db *gorm.DB
}

// NewBuilder creates a new quote builder
func NewBuilder(db *gorm.DB) *Builder {
	return &Builder{db: db}
}

// BuildResult contains the built quote entries and metadata
type BuildResult struct {
	Entries []CacheEntry
	ChatID  int64
}

// BuildFrom builds a quote thread starting from a message ID by recursively
// following reply chains through the cache.
// This ports the Quotes.Builder.build_from functionality from Elixir.
func (b *Builder) BuildFrom(ctx context.Context, chatID int64, messageID int64) (*BuildResult, error) {
	var entries []CacheEntry
	currentID := messageID

	// Recursively follow reply chains
	for currentID != 0 {
		var entry CacheEntry
		err := b.db.WithContext(ctx).
			Where("chat_id = ? AND message_id = ?", chatID, currentID).
			First(&entry).Error

		if err != nil {
			if err == gorm.ErrRecordNotFound {
				// Message not in cache, stop building
				break
			}
			return nil, fmt.Errorf("failed to fetch cache entry: %w", err)
		}

		// Prepend entry (we're building from newest to oldest, but want oldest first)
		entries = append([]CacheEntry{entry}, entries...)

		// Follow reply chain
		if entry.ReplyID != nil && *entry.ReplyID != 0 {
			currentID = *entry.ReplyID
		} else {
			break
		}
	}

	if len(entries) == 0 {
		return nil, fmt.Errorf("no cache entries found for message %d in chat %d", messageID, chatID)
	}

	return &BuildResult{
		Entries: entries,
		ChatID:  chatID,
	}, nil
}

// BuildFromMessage builds a quote from a Telegram message structure directly
// This is used when we have the message but need to build the full thread
func (b *Builder) BuildFromMessage(ctx context.Context, chatID int64, messageID int64, replyToMessageID *int64) (*BuildResult, error) {
	// First try to build from cache
	result, err := b.BuildFrom(ctx, chatID, messageID)
	if err == nil {
		return result, nil
	}

	// If not in cache and we have a reply, try to build from the reply chain
	if replyToMessageID != nil && *replyToMessageID != 0 {
		return b.BuildFrom(ctx, chatID, *replyToMessageID)
	}

	return nil, err
}

// ExtractMessage extracts the message map from a cache entry's JSON
type MessageData struct {
	MessageID int64                  `json:"message_id"`
	From      map[string]interface{} `json:"from"`
	Date      int64                  `json:"date"`
	Text      string                 `json:"text"`
	Chat      map[string]interface{} `json:"chat"`
}

// ExtractMessageData extracts message data from a cache entry
func ExtractMessageData(entry CacheEntry) (*MessageData, error) {
	var data MessageData
	if err := json.Unmarshal(entry.Message, &data); err != nil {
		return nil, fmt.Errorf("failed to unmarshal message: %w", err)
	}
	return &data, nil
}
