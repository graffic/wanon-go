package quotes

import (
	"context"

	"github.com/go-telegram/bot/models"
	"github.com/graffic/wanon-go/internal/bot"
)

// TelegramClientAdapter adapts the telegram.Client to quotes.TelegramClient interface
type TelegramClientAdapter struct {
	client sendTextClient
}

// sendTextClient is the minimal interface needed from telegram.Client
type sendTextClient interface {
	SendText(ctx context.Context, chatID int64, text string) (*models.Message, error)
}

// NewTelegramClientAdapter creates a new adapter for the telegram client
func NewTelegramClientAdapter(client sendTextClient) *TelegramClientAdapter {
	return &TelegramClientAdapter{client: client}
}

// SendMessage implements the quotes.TelegramClient interface
func (a *TelegramClientAdapter) SendMessage(ctx context.Context, chatID int64, text string) error {
	_, err := a.client.SendText(ctx, chatID, text)
	return err
}

// CommandAdapter adapts a quotes handler to the bot.Command interface
type CommandAdapter struct {
	handler handler
}

// handler is the interface that quote handlers implement
type handler interface {
	CanHandle(message *TelegramMessage) bool
	Handle(ctx context.Context, message *TelegramMessage) error
}

// NewCommandAdapter creates a new command adapter
func NewCommandAdapter(h handler) *CommandAdapter {
	return &CommandAdapter{handler: h}
}

// Execute implements the bot.Command interface
func (a *CommandAdapter) Execute(ctx context.Context, msg *models.Message) error {
	// Convert models.Message to TelegramMessage
	tgMsg := convertToTelegramMessage(msg)
	return a.handler.Handle(ctx, tgMsg)
}

// convertToTelegramMessage converts a models.Message to TelegramMessage
func convertToTelegramMessage(msg *models.Message) *TelegramMessage {
	if msg == nil {
		return nil
	}

	tgMsg := &TelegramMessage{
		MessageID: int64(msg.ID),
		Text:      msg.Text,
		Chat: map[string]interface{}{
			"id": msg.Chat.ID,
		},
	}

	// Add optional Chat fields
	if msg.Chat.Title != "" {
		tgMsg.Chat["title"] = msg.Chat.Title
	}
	if msg.Chat.Type != "" {
		tgMsg.Chat["type"] = msg.Chat.Type
	}

	// Convert From
	if msg.From != nil {
		tgMsg.From = map[string]interface{}{
			"id":         msg.From.ID,
			"first_name": msg.From.FirstName,
		}
		if msg.From.LastName != "" {
			tgMsg.From["last_name"] = msg.From.LastName
		}
		if msg.From.Username != "" {
			tgMsg.From["username"] = msg.From.Username
		}
	}

	// Convert ReplyToMessage
	if msg.ReplyToMessage != nil {
		tgMsg.ReplyToMessage = convertToTelegramMessage(msg.ReplyToMessage)
	}

	return tgMsg
}

// Ensure CommandAdapter implements bot.Command
var _ bot.Command = (*CommandAdapter)(nil)

// Ensure TelegramClientAdapter implements TelegramClient
var _ TelegramClient = (*TelegramClientAdapter)(nil)
