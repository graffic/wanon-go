package quotes

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"gorm.io/gorm"
)

// RQuoteHandler handles the /rquote command
// This ports the Quotes.RQuote functionality from Elixir
type RQuoteHandler struct {
	db       *gorm.DB
	store    *Store
	renderer *Renderer
}

// NewRQuoteHandler creates a new rquote handler
func NewRQuoteHandler(db *gorm.DB) *RQuoteHandler {
	return &RQuoteHandler{
		db:       db,
		store:    NewStore(db),
		renderer: NewRenderer(),
	}
}

// Handle processes the /rquote command
// This signature matches go-telegram/bot handler func
func (h *RQuoteHandler) Handle(ctx context.Context, b *bot.Bot, update *models.Update) error {
	msg := update.Message
	if msg == nil {
		return nil
	}

	chatID := msg.Chat.ID
	slog.Info("executing /rquote command", "chat_id", chatID, "user_id", msg.From.ID)

	// Check if there are any quotes for this chat
	count, err := h.store.CountForChat(ctx, chatID)
	if err != nil {
		return fmt.Errorf("failed to count quotes: %w", err)
	}

	if count == 0 {
		_, err := b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "No quotes found in this chat. Add some with /addquote!",
		})
		return err
	}

	// Get a random quote for this chat
	quote, err := h.store.GetRandomForChat(ctx, chatID)
	if err != nil {
		return fmt.Errorf("failed to get random quote: %w", err)
	}

	if quote == nil {
		_, err := b.SendMessage(ctx, &bot.SendMessageParams{
			ChatID: chatID,
			Text:   "No quotes found in this chat.",
		})
		return err
	}

	// Render the quote
	rendered, err := h.renderer.RenderWithDate(quote)
	if err != nil {
		return fmt.Errorf("failed to render quote: %w", err)
	}

	// Send the quote
	_, err = b.SendMessage(ctx, &bot.SendMessageParams{
		ChatID: chatID,
		Text:   rendered,
	})
	return err
}

// Command returns the command name
func (h *RQuoteHandler) Command() string {
	return "/rquote"
}

// Description returns the command description
func (h *RQuoteHandler) Description() string {
	return "Get a random quote from this chat"
}
