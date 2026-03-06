package quotes

import (
	"context"
	"log/slog"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"gorm.io/gorm"
)

// RQuoteHandler handles the /rquote command
// This ports the Quotes.RQuote functionality from Elixir
type RQuoteHandler struct {
	b        *bot.Bot
	db       *gorm.DB
	store    *Store
	renderer *Renderer
}

// NewRQuoteHandler creates a new rquote handler
func NewRQuoteHandler(b *bot.Bot, db *gorm.DB) *RQuoteHandler {
	return &RQuoteHandler{
		b:        b,
		db:       db,
		store:    NewStore(db),
		renderer: NewRenderer(),
	}
}

// Handle processes the /rquote command
// This signature matches bot.HandlerFunc
func (h *RQuoteHandler) Handle(ctx context.Context, _ *bot.Bot, update *models.Update) {
	msg := update.Message
	if msg == nil {
		return
	}

	chatID := msg.Chat.ID
	slog.Info("executing /rquote command", "chat_id", chatID, "user_id", msg.From.ID)

	// Check if there are any quotes for this chat
	count, err := h.store.CountForChat(ctx, chatID)
	if err != nil {
		slog.Error("failed to count quotes", "error", err)
		return
	}

	if count == 0 {
		_, err := h.b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "No quotes found in this chat. Add some with /addquote!",
		})
		if err != nil {
			slog.Error("failed to send message", "error", err)
		}
		return
	}

	// Get a random quote for this chat
	quote, err := h.store.GetRandomForChat(ctx, chatID)
	if err != nil {
		slog.Error("failed to get random quote", "error", err)
		return
	}

	if quote == nil {
		_, err := h.b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "No quotes found in this chat.",
		})
		if err != nil {
			slog.Error("failed to send message", "error", err)
		}
		return
	}

	// Render the quote
	rendered, err := h.renderer.RenderWithDate(quote)
	if err != nil {
		slog.Error("failed to render quote", "error", err)
		return
	}

	// Send the quote
	_, err = h.b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: chatID,
		Text:   rendered,
	})
	if err != nil {
		slog.Error("failed to send message", "error", err)
	}
}

// Command returns the command name
func (h *RQuoteHandler) Command() string {
	return "/rquote"
}

// Description returns the command description
func (h *RQuoteHandler) Description() string {
	return "Get a random quote from this chat"
}
