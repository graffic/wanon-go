package cache

import (
	"context"
	"encoding/json"
	"log/slog"

	"gorm.io/datatypes"
)

// AddCommand handles adding messages to the cache
type AddCommand struct {
	service *Service
	logger  *slog.Logger
}

// NewAddCommand creates a new add command handler
func NewAddCommand(service *Service, logger *slog.Logger) *AddCommand {
	return &AddCommand{
		service: service,
		logger:  logger,
	}
}

// Execute processes a message and adds it to the cache
func (c *AddCommand) Execute(ctx context.Context, rawMessage json.RawMessage) error {
	var msg Message
	if err := json.Unmarshal(rawMessage, &msg); err != nil {
		c.logger.Error("failed to unmarshal message", "error", err)
		return err
	}

	// Store the raw message for later use
	msg.Raw = rawMessage

	c.logger.Debug("adding message to cache",
		"chat_id", msg.Chat.ID,
		"message_id", msg.MessageID,
		"date", msg.Date,
	)

	entry := &CacheEntry{
		ChatID:    msg.Chat.ID,
		MessageID: msg.MessageID,
		Date:      msg.Date,
	}

	if msg.ReplyTo != nil {
		entry.ReplyID = &msg.ReplyTo.MessageID
	}

	// Store the full message as JSON
	messageJSON, err := json.Marshal(msg)
	if err != nil {
		c.logger.Error("failed to marshal message", "error", err)
		return err
	}
	entry.Message = datatypes.JSON(messageJSON)

	// Upsert: insert or update if conflict
	err = c.service.db.WithContext(ctx).
		Where("chat_id = ? AND message_id = ?", entry.ChatID, entry.MessageID).
		Assign(map[string]interface{}{
			"reply_id": entry.ReplyID,
			"date":     entry.Date,
			"message":  entry.Message,
		}).
		FirstOrCreate(entry).Error

	if err != nil {
		c.logger.Error("failed to add message to cache", "error", err)
		return err
	}

	c.logger.Debug("message added to cache successfully",
		"chat_id", msg.Chat.ID,
		"message_id", msg.MessageID,
	)

	return nil
}

// ShouldHandle returns true if this command should handle the message
func (c *AddCommand) ShouldHandle(msg *Message) bool {
	// Add command handles all regular messages
	return msg.MessageID != 0 && msg.Chat.ID != 0
}
