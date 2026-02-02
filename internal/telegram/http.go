package telegram

import (
	"context"
	"fmt"
	"os"
	"sync"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

// HTTPClient is now a wrapper around go-telegram/bot.Bot
// It implements the Client interface
type HTTPClient struct {
	bot        *bot.Bot
	updatesCh  chan []models.Update
	mu         sync.RWMutex
	handlers   []func(ctx context.Context, update *models.Update)
}

// NewHTTPClient creates a new HTTP-based Telegram client using go-telegram/bot
func NewHTTPClient(token string, opts ...Option) (*HTTPClient, error) {
	// Set default options
	options := &clientOptions{
		debug: os.Getenv("DEBUG") == "true",
	}

	// Apply options
	for _, opt := range opts {
		opt(options)
	}

	// Create bot options
	botOpts := []bot.Option{
		bot.WithSkipGetMe(), // Skip getMe call during initialization for testing
	}
	if options.debug {
		botOpts = append(botOpts, bot.WithDebug())
	}

	// Create a channel for updates
	updatesCh := make(chan []models.Update, 100)

	// Create the HTTPClient first (without bot)
	client := &HTTPClient{
		updatesCh: updatesCh,
		handlers:  make([]func(ctx context.Context, update *models.Update), 0),
	}

	// Add a default handler that captures all updates
	botOpts = append(botOpts, bot.WithDefaultHandler(func(ctx context.Context, b *bot.Bot, update *models.Update) {
		client.handleUpdate(ctx, update)
	}))

	// Create the bot
	b, err := bot.New(token, botOpts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create bot: %w", err)
	}

	client.bot = b
	return client, nil
}

// handleUpdate processes a single update from the bot library
func (c *HTTPClient) handleUpdate(ctx context.Context, update *models.Update) {
	// Send to channel
	select {
	case c.updatesCh <- []models.Update{*update}:
	default:
		// Channel is full, drop the update
	}

	// Call registered handlers
	c.mu.RLock()
	handlers := make([]func(ctx context.Context, update *models.Update), len(c.handlers))
	copy(handlers, c.handlers)
	c.mu.RUnlock()

	for _, handler := range handlers {
		handler(ctx, update)
	}
}

// clientOptions holds configuration options
type clientOptions struct {
	debug bool
}

// Option configures the HTTPClient
type Option func(*clientOptions)

// WithDebug enables debug mode
func WithDebug() Option {
	return func(c *clientOptions) {
		c.debug = true
	}
}

// GetMe implements the Client interface
func (c *HTTPClient) GetMe(ctx context.Context) (*models.User, error) {
	return c.bot.GetMe(ctx)
}

// GetUpdates fetches updates from Telegram
// Note: Since go-telegram/bot manages polling internally, this method
// returns updates from the internal channel. The offset, limit, and timeout
// parameters are maintained for API compatibility but don't affect behavior.
func (c *HTTPClient) GetUpdates(ctx context.Context, offset int, limit int, timeout int) ([]models.Update, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case updates := <-c.updatesCh:
		return updates, nil
	}
}

// SendMessage implements the Client interface
func (c *HTTPClient) SendMessage(ctx context.Context, chatID int64, text string, replyToMessageID *int64) (*models.Message, error) {
	params := &bot.SendMessageParams{
		ChatID: chatID,
		Text:   text,
	}
	if replyToMessageID != nil {
		params.ReplyParameters = &models.ReplyParameters{
			MessageID: int(*replyToMessageID),
		}
	}
	return c.bot.SendMessage(ctx, params)
}

// SendText implements the Client interface
func (c *HTTPClient) SendText(ctx context.Context, chatID int64, text string) (*models.Message, error) {
	return c.SendMessage(ctx, chatID, text, nil)
}

// ReplyToMessage implements the Client interface
func (c *HTTPClient) ReplyToMessage(ctx context.Context, chatID int64, messageID int64, text string) (*models.Message, error) {
	return c.SendMessage(ctx, chatID, text, &messageID)
}

// SetWebhook implements the Client interface
func (c *HTTPClient) SetWebhook(ctx context.Context, url string) error {
	params := &bot.SetWebhookParams{
		URL: url,
	}
	_, err := c.bot.SetWebhook(ctx, params)
	return err
}

// DeleteWebhook implements the Client interface
func (c *HTTPClient) DeleteWebhook(ctx context.Context) error {
	params := &bot.DeleteWebhookParams{
		DropPendingUpdates: false,
	}
	_, err := c.bot.DeleteWebhook(ctx, params)
	return err
}

// GetChat implements the Client interface
func (c *HTTPClient) GetChat(ctx context.Context, chatID int64) (*models.ChatFullInfo, error) {
	params := &bot.GetChatParams{
		ChatID: chatID,
	}
	return c.bot.GetChat(ctx, params)
}

// GetChatAdministrators implements the Client interface
func (c *HTTPClient) GetChatAdministrators(ctx context.Context, chatID int64) ([]models.ChatMember, error) {
	params := &bot.GetChatAdministratorsParams{
		ChatID: chatID,
	}
	return c.bot.GetChatAdministrators(ctx, params)
}

// Start begins the bot's polling loop
// This should be called in a goroutine
func (c *HTTPClient) Start(ctx context.Context) error {
	c.bot.Start(ctx)
	return ctx.Err()
}

// RegisterHandler adds a handler for updates
func (c *HTTPClient) RegisterHandler(handler func(ctx context.Context, update *models.Update)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.handlers = append(c.handlers, handler)
}
