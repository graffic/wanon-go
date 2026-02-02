package cache

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/go-telegram/bot/models"
)

// Middleware provides cache integration for the dispatcher
type Middleware struct {
	addCommand  *AddCommand
	editCommand *EditCommand
	logger      *slog.Logger
}

// NewMiddleware creates a new cache middleware
func NewMiddleware(service *Service, logger *slog.Logger) *Middleware {
	return &Middleware{
		addCommand:  NewAddCommand(service, logger),
		editCommand: NewEditCommand(service, logger),
		logger:      logger,
	}
}

// HandleUpdate processes an update through the cache
// This should be registered with the dispatcher's AddUpdateHandler
func (m *Middleware) HandleUpdate(ctx context.Context, update *models.Update) error {
	// Handle regular messages
	if update.Message != nil {
		return m.handleMessage(ctx, update.Message)
	}

	// Handle edited messages
	if update.EditedMessage != nil {
		return m.handleEditedMessage(ctx, update.EditedMessage)
	}

	return nil
}

// handleMessage processes a regular message and adds it to cache
func (m *Middleware) handleMessage(ctx context.Context, msg *models.Message) error {
	// Convert to JSON for the AddCommand
	msgData := map[string]interface{}{
		"message_id": msg.ID,
		"chat": map[string]interface{}{
			"id":   msg.Chat.ID,
			"type": msg.Chat.Type,
		},
		"date": msg.Date,
	}

	if msg.Text != "" {
		msgData["text"] = msg.Text
	}

	if msg.From != nil {
		msgData["from"] = map[string]interface{}{
			"id":         msg.From.ID,
			"first_name": msg.From.FirstName,
		}
		if msg.From.LastName != "" {
			msgData["from"].(map[string]interface{})["last_name"] = msg.From.LastName
		}
		if msg.From.Username != "" {
			msgData["from"].(map[string]interface{})["username"] = msg.From.Username
		}
	}

	if msg.ReplyToMessage != nil {
		msgData["reply_to_message"] = map[string]interface{}{
			"message_id": msg.ReplyToMessage.ID,
		}
	}

	rawJSON, err := json.Marshal(msgData)
	if err != nil {
		m.logger.Error("failed to marshal message for cache", "error", err)
		return err
	}

	return m.addCommand.Execute(ctx, rawJSON)
}

// handleEditedMessage processes an edited message and updates the cache
func (m *Middleware) handleEditedMessage(ctx context.Context, msg *models.Message) error {
	// Convert to JSON for the EditCommand
	msgData := map[string]interface{}{
		"message_id": msg.ID,
		"chat": map[string]interface{}{
			"id":   msg.Chat.ID,
			"type": msg.Chat.Type,
		},
		"date":      msg.Date,
		"edit_date": msg.EditDate,
	}

	if msg.Text != "" {
		msgData["text"] = msg.Text
	}

	if msg.From != nil {
		msgData["from"] = map[string]interface{}{
			"id":         msg.From.ID,
			"first_name": msg.From.FirstName,
		}
		if msg.From.LastName != "" {
			msgData["from"].(map[string]interface{})["last_name"] = msg.From.LastName
		}
		if msg.From.Username != "" {
			msgData["from"].(map[string]interface{})["username"] = msg.From.Username
		}
	}

	rawJSON, err := json.Marshal(msgData)
	if err != nil {
		m.logger.Error("failed to marshal edited message for cache", "error", err)
		return err
	}

	return m.editCommand.Execute(ctx, rawJSON)
}
