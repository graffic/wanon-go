package quotes

import (
	"context"
	"fmt"
	"strings"

	"gorm.io/gorm"
)

// RQuoteHandler handles the /rquote command
// This ports the Quotes.RQuote functionality from Elixir
type RQuoteHandler struct {
	db       *gorm.DB
	store    *Store
	renderer *Renderer
	client   TelegramClient
}

// NewRQuoteHandler creates a new rquote handler
func NewRQuoteHandler(db *gorm.DB, client TelegramClient) *RQuoteHandler {
	return &RQuoteHandler{
		db:       db,
		store:    NewStore(db),
		renderer: NewRenderer(),
		client:   client,
	}
}

// CanHandle checks if this handler can process the message
func (h *RQuoteHandler) CanHandle(message *TelegramMessage) bool {
	if message == nil || message.Text == "" {
		return false
	}
	
	// Check if text starts with /rquote (case insensitive)
	text := strings.TrimSpace(message.Text)
	return strings.HasPrefix(strings.ToLower(text), "/rquote")
}

// Handle processes the /rquote command
func (h *RQuoteHandler) Handle(ctx context.Context, message *TelegramMessage) error {
	chatID := h.extractChatID(message)
	if chatID == 0 {
		return fmt.Errorf("could not extract chat ID from message")
	}

	// Check if there are any quotes for this chat
	count, err := h.store.CountForChat(ctx, chatID)
	if err != nil {
		return fmt.Errorf("failed to count quotes: %w", err)
	}

	if count == 0 {
		return h.client.SendMessage(ctx, chatID, "No quotes found in this chat. Add some with /addquote!")
	}

	// Get a random quote for this chat
	quote, err := h.store.GetRandomForChat(ctx, chatID)
	if err != nil {
		return fmt.Errorf("failed to get random quote: %w", err)
	}

	if quote == nil {
		return h.client.SendMessage(ctx, chatID, "No quotes found in this chat.")
	}

	// Render the quote
	rendered, err := h.renderer.RenderWithDate(quote)
	if err != nil {
		return fmt.Errorf("failed to render quote: %w", err)
	}

	// Send the quote
	return h.client.SendMessage(ctx, chatID, rendered)
}

// extractChatID extracts the chat ID from a message
func (h *RQuoteHandler) extractChatID(message *TelegramMessage) int64 {
	if message.Chat == nil {
		return 0
	}
	
	// Try to get id from chat map
	if id, ok := message.Chat["id"].(float64); ok {
		return int64(id)
	}
	if id, ok := message.Chat["id"].(int64); ok {
		return id
	}
	
	return 0
}

// Command returns the command name
func (h *RQuoteHandler) Command() string {
	return "/rquote"
}

// Description returns the command description
func (h *RQuoteHandler) Description() string {
	return "Get a random quote from this chat"
}
