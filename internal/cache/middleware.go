package cache

import (
	"context"
	"log/slog"

	"github.com/go-telegram/bot/models"
)

// Middleware provides cache integration for the dispatcher
type Middleware struct {
	service *Service
	logger  *slog.Logger
}

// NewMiddleware creates a new cache middleware
func NewMiddleware(service *Service, logger *slog.Logger) *Middleware {
	return &Middleware{
		service: service,
		logger:  logger,
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

func mapMessage(msg *models.Message) *Message {
	res := &Message{
		MessageID: int64(msg.ID),
		Chat: Chat{
			ID:   msg.Chat.ID,
			Type: string(msg.Chat.Type),
		},
		Date: int64(msg.Date),
		Text: msg.Text,
	}

	if msg.From != nil {
		res.From = &User{
			ID:        msg.From.ID,
			FirstName: msg.From.FirstName,
			LastName:  msg.From.LastName,
			Username:  msg.From.Username,
		}
	}

	if msg.ReplyToMessage != nil {
		res.ReplyTo = &Message{
			MessageID: int64(msg.ReplyToMessage.ID),
		}
	}

	return res
}

// handleMessage processes a regular message and adds it to cache
func (m *Middleware) handleMessage(ctx context.Context, msg *models.Message) error {
	mappedMsg := mapMessage(msg)

	m.logger.Debug("adding message to cache",
		"chat_id", mappedMsg.Chat.ID,
		"message_id", mappedMsg.MessageID,
		"date", mappedMsg.Date,
	)

	return m.service.Add(ctx, mappedMsg)
}

// handleEditedMessage processes an edited message and updates the cache
func (m *Middleware) handleEditedMessage(ctx context.Context, msg *models.Message) error {
	mappedMsg := mapMessage(msg)

	m.logger.Debug("processing edited message",
		"chat_id", mappedMsg.Chat.ID,
		"message_id", mappedMsg.MessageID,
		"date", mappedMsg.Date,
	)

	return m.service.Edit(ctx, mappedMsg)
}

