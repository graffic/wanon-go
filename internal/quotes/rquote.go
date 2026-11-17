package quotes

import (
	"context"
	"errors"
	"log/slog"
	"strconv"
	"strings"
	"unicode/utf8"

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
		store:       NewStore(db),
		renderer:    NewRenderer(),
	}
}

// Handle processes the /rquote command
// This signature matches bot.HandlerFunc
// Usage: /rquote - returns a random quote
// Usage: /rquote <id> or /rquote #<id> - looks up a quote in this chat by ID
// Usage: /rquote <search text> - returns a random quote containing the text
func (h *RQuoteHandler) Handle(ctx context.Context, _ *bot.Bot, update *models.Update) {
	msg := update.Message
	if msg == nil {
		return
	}

	chatID := msg.Chat.ID
	searchText := h.extractSearchText(msg.Text)
	if utf8.RuneCountInString(searchText) > 200 {
		h.Reply(ctx, chatID, "Search input must be at most 200 characters.")
		return
	}

	if searchText != "" {
		h.Logger.Info("executing /rquote search", "chat_id", chatID, "user_id", msg.From.ID, "query", searchText)
	} else {
		h.Logger.Info("executing /rquote command", "chat_id", chatID, "user_id", msg.From.ID)
	}

	var quote *Quote
	var err error

	if searchText != "" {
		quote, err = h.searchQuote(ctx, chatID, searchText)
		if errors.Is(err, gorm.ErrRecordNotFound) {
			h.Reply(ctx, chatID, "Quote #"+strings.TrimPrefix(searchText, "#")+" not found in this chat.")
			return
		}
		if err != nil {
			h.Logger.Error("failed to search quotes", "error", err, "search_text", searchText)
			return
		}

		if quote == nil {
			h.Reply(ctx, chatID, "No quotes found containing: "+searchText)
			return
		}
	} else {
		// Get random quote (original behavior)
		count, countErr := h.store.CountForChat(ctx, chatID)
		if countErr != nil {
			h.Logger.Error("failed to count quotes", "error", countErr)
			return
		}

		if count == 0 {
			h.Reply(ctx, chatID, "No quotes found in this chat. Add some with /addquote!")
			return
		}

		quote, err = h.store.GetRandomForChat(ctx, chatID)
	}

	if err != nil {
		h.Logger.Error("failed to get quote", "error", err, "search_text", searchText)
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

// searchQuote looks up numeric input as a quote ID; other input uses text search.
func (h *RQuoteHandler) searchQuote(ctx context.Context, chatID int64, searchText string) (*Quote, error) {
	idText := strings.TrimPrefix(searchText, "#")
	if idText == "" || strings.ContainsFunc(idText, func(r rune) bool { return r < '0' || r > '9' }) {
		return h.store.SearchByText(ctx, chatID, searchText)
	}

	// Quote IDs are stored as PostgreSQL BIGINT, which is a signed 64-bit integer.
	id, err := strconv.ParseUint(idText, 10, 63)
	if err != nil {
		return nil, gorm.ErrRecordNotFound
	}
	quote, err := h.store.GetByID(ctx, uint(id))
	if err != nil {
		return nil, err
	}
	if quote.ChatID != chatID {
		return nil, gorm.ErrRecordNotFound
	}
	return quote, nil
}

// extractSearchText extracts the search text from the command message
// Returns empty string if no search text provided
func (h *RQuoteHandler) extractSearchText(text string) string {
	// Remove the command prefix
	parts := strings.SplitN(text, " ", 2)
	if len(parts) < 2 {
		return ""
	}

	searchText := strings.TrimSpace(parts[1])
	return searchText
}

// Command returns the command name
func (h *RQuoteHandler) Command() string {
	return "/rquote"
}

// Description returns the command description
func (h *RQuoteHandler) Description() string {
	return "Get a quote from this chat. Usage: /rquote [quote ID, #ID, or search text]"
}
