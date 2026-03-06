package quotes

import (
	"context"
	"log/slog"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	internalBot "github.com/graffic/wanon-go/internal/bot"
	"gorm.io/gorm"
)

// RQuoteHandler handles the /rquote command
type RQuoteHandler struct {
	internalBot.BaseHandler
	db       *gorm.DB
	store    *Store
	renderer *Renderer
}

func NewRQuoteHandler(b internalBot.BotClient, db *gorm.DB, logger *slog.Logger) *RQuoteHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &RQuoteHandler{
		BaseHandler: internalBot.NewBaseHandler(b, logger),
		db:          db,
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
	h.Logger.Info("executing /rquote command", "chat_id", chatID, "user_id", msg.From.ID)

	// Check if there are any quotes for this chat
	count, err := h.store.CountForChat(ctx, chatID)
	if err != nil {
		h.Logger.Error("failed to count quotes", "error", err)
		return
	}

	if count == 0 {
		h.Reply(ctx, chatID, "No quotes found in this chat. Add some with /addquote!")
		return
	}

	// Get a random quote for this chat
	quote, err := h.store.GetRandomForChat(ctx, chatID)
	if err != nil {
		h.Logger.Error("failed to get random quote", "error", err)
		return
	}

	if quote == nil {
		h.Reply(ctx, chatID, "No quotes found in this chat.")
		return
	}

	// Render the quote
	rendered, err := h.renderer.RenderWithDate(quote)
	if err != nil {
		h.Logger.Error("failed to render quote", "error", err)
		return
	}

	// Send the quote
	h.Reply(ctx, chatID, rendered)
}

// Command returns the command name
func (h *RQuoteHandler) Command() string {
	return "/rquote"
}

// Description returns the command description
func (h *RQuoteHandler) Description() string {
	return "Get a random quote from this chat"
}
