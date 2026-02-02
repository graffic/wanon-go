package telegram

import (
	"context"

	"github.com/go-telegram/bot/models"
)

// Client defines the interface for Telegram bot operations
// This wraps the go-telegram/bot library
 type Client interface {
	// GetMe returns information about the bot
	GetMe(ctx context.Context) (*models.User, error)

	// GetUpdates fetches updates from Telegram
	GetUpdates(ctx context.Context, offset int, limit int, timeout int) ([]models.Update, error)

	// SendMessage sends a message to a chat
	SendMessage(ctx context.Context, chatID int64, text string, replyToMessageID *int64) (*models.Message, error)

	// SendText sends a simple text message to a chat
	SendText(ctx context.Context, chatID int64, text string) (*models.Message, error)

	// ReplyToMessage sends a reply to a specific message
	ReplyToMessage(ctx context.Context, chatID int64, messageID int64, text string) (*models.Message, error)

	// SetWebhook configures the webhook URL
	SetWebhook(ctx context.Context, url string) error

	// DeleteWebhook removes the webhook configuration
	DeleteWebhook(ctx context.Context) error

	// GetChat retrieves information about a chat
	GetChat(ctx context.Context, chatID int64) (*models.ChatFullInfo, error)

	// GetChatAdministrators retrieves the list of administrators in a chat
	GetChatAdministrators(ctx context.Context, chatID int64) ([]models.ChatMember, error)
}

// Command represents a bot command
type Command struct {
	Command     string `json:"command"`
	Description string `json:"description"`
}

// CommandHandler is a function that handles bot commands
 type CommandHandler func(ctx context.Context, client Client, message *models.Message) error

// Middleware is a function that wraps command handlers
type Middleware func(CommandHandler) CommandHandler
