package cache

import (
	"context"
	"encoding/json"
	"log/slog"

	"gorm.io/datatypes"
	"gorm.io/gorm"
)

// EditCommand handles editing messages in the cache
type EditCommand struct {
	service *Service
	logger  *slog.Logger
}

// NewEditCommand creates a new edit command handler
func NewEditCommand(service *Service, logger *slog.Logger) *EditCommand {
	return &EditCommand{
		service: service,
		logger:  logger,
	}
}

// EditedMessage represents a message edit from Telegram
type EditedMessage struct {
	MessageID int64  `json:"message_id"`
	Chat      Chat   `json:"chat"`
	Date      int64  `json:"date"`
	EditDate  int64  `json:"edit_date"`
	Text      string `json:"text,omitempty"`
	From      *User  `json:"from,omitempty"`
}

// Execute processes an edited message and updates it in the cache
func (c *EditCommand) Execute(ctx context.Context, rawMessage json.RawMessage) error {
	var editedMsg EditedMessage
	if err := json.Unmarshal(rawMessage, &editedMsg); err != nil {
		c.logger.Error("failed to unmarshal edited message", "error", err)
		return err
	}

	c.logger.Debug("processing edited message",
		"chat_id", editedMsg.Chat.ID,
		"message_id", editedMsg.MessageID,
		"edit_date", editedMsg.EditDate,
	)

	// Find the existing cache entry
	var entry CacheEntry
	result := c.service.db.WithContext(ctx).
		Where("chat_id = ? AND message_id = ?", editedMsg.Chat.ID, editedMsg.MessageID).
		First(&entry)

	if result.Error == gorm.ErrRecordNotFound {
		c.logger.Debug("edited message not found in cache, skipping",
			"chat_id", editedMsg.Chat.ID,
			"message_id", editedMsg.MessageID,
		)
		return nil
	}
	if result.Error != nil {
		c.logger.Error("failed to find message in cache", "error", result.Error)
		return result.Error
	}

	// Parse the existing message
	var existingMsg Message
	if err := json.Unmarshal(entry.Message, &existingMsg); err != nil {
		c.logger.Error("failed to unmarshal existing message", "error", err)
		return err
	}

	// Update the message fields
	existingMsg.Text = editedMsg.Text
	if editedMsg.From != nil {
		existingMsg.From = editedMsg.From
	}

	// Marshal the updated message
	updatedJSON, err := json.Marshal(existingMsg)
	if err != nil {
		c.logger.Error("failed to marshal updated message", "error", err)
		return err
	}

	// Update the cache entry
	err = c.service.db.WithContext(ctx).
		Model(&entry).
		Updates(map[string]interface{}{
			"message": datatypes.JSON(updatedJSON),
		}).Error

	if err != nil {
		c.logger.Error("failed to update message in cache", "error", err)
		return err
	}

	c.logger.Debug("message updated in cache successfully",
		"chat_id", editedMsg.Chat.ID,
		"message_id", editedMsg.MessageID,
	)

	return nil
}

// ShouldHandle returns true if this command should handle the message
func (c *EditCommand) ShouldHandle(msg *EditedMessage) bool {
	// Edit command handles edited messages
	return msg.MessageID != 0 && msg.Chat.ID != 0
}
