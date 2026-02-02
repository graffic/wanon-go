package quotes

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"gorm.io/gorm"
)

// TelegramMessage represents a simplified Telegram message structure
type TelegramMessage struct {
	MessageID       int64                  `json:"message_id"`
	From            map[string]interface{} `json:"from"`
	Chat            map[string]interface{} `json:"chat"`
	Text            string                 `json:"text"`
	ReplyToMessage  *TelegramMessage       `json:"reply_to_message"`
}

// TelegramClient interface for sending messages
type TelegramClient interface {
	SendMessage(ctx context.Context, chatID int64, text string) error
}

// AddQuoteHandler handles the /addquote command
// This ports the Quotes.AddQuote functionality from Elixir
type AddQuoteHandler struct {
	db      *gorm.DB
	builder *Builder
	store   *Store
	client  TelegramClient
}

// NewAddQuoteHandler creates a new addquote handler
func NewAddQuoteHandler(db *gorm.DB, client TelegramClient) *AddQuoteHandler {
	return &AddQuoteHandler{
		db:      db,
		builder: NewBuilder(db),
		store:   NewStore(db),
		client:  client,
	}
}

// CanHandle checks if this handler can process the message
func (h *AddQuoteHandler) CanHandle(message *TelegramMessage) bool {
	if message == nil || message.Text == "" {
		return false
	}
	
	// Check if text starts with /addquote (case insensitive)
	text := strings.TrimSpace(message.Text)
	return strings.HasPrefix(strings.ToLower(text), "/addquote")
}

// Handle processes the /addquote command
func (h *AddQuoteHandler) Handle(ctx context.Context, message *TelegramMessage) error {
	chatID := h.extractChatID(message)
	if chatID == 0 {
		return fmt.Errorf("could not extract chat ID from message")
	}

	// Check if message is a reply
	if message.ReplyToMessage == nil {
		return h.client.SendMessage(ctx, chatID, "Please reply to a message to add it as a quote.")
	}

	// Build the quote from cache
	replyMsg := message.ReplyToMessage
	result, err := h.builder.BuildFrom(ctx, chatID, replyMsg.MessageID)
	if err != nil {
		// If not in cache, try to use the reply message directly
		// This handles the case where the message is recent but cache missed
		result, err = h.buildFromReplyMessage(replyMsg)
		if err != nil {
			return h.client.SendMessage(ctx, chatID, "Could not build quote. The message may be too old or not in cache.")
		}
	}

	// Store the quote
	creator := message.From
	if creator == nil {
		creator = map[string]interface{}{"id": 0, "first_name": "Unknown"}
	}

	quote, err := h.store.StoreFromBuild(ctx, creator, result)
	if err != nil {
		return fmt.Errorf("failed to store quote: %w", err)
	}

	// Send confirmation
	confirmation := fmt.Sprintf("Quote #%d added with %d entries!", quote.ID, len(quote.Entries))
	return h.client.SendMessage(ctx, chatID, confirmation)
}

// buildFromReplyMessage builds a quote result from a reply message directly
// This is a fallback when the message is not in cache
func (h *AddQuoteHandler) buildFromReplyMessage(replyMsg *TelegramMessage) (*BuildResult, error) {
	// Convert message to JSON
	msgJSON, err := json.Marshal(replyMsg)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal message: %w", err)
	}

	chatID := h.extractChatID(replyMsg)
	
	entry := CacheEntry{
		ChatID:    chatID,
		MessageID: replyMsg.MessageID,
		Message:   msgJSON,
	}

	return &BuildResult{
		Entries: []CacheEntry{entry},
		ChatID:  chatID,
	}, nil
}

// extractChatID extracts the chat ID from a message
func (h *AddQuoteHandler) extractChatID(message *TelegramMessage) int64 {
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
func (h *AddQuoteHandler) Command() string {
	return "/addquote"
}

// Description returns the command description
func (h *AddQuoteHandler) Description() string {
	return "Add a quote by replying to a message"
}
