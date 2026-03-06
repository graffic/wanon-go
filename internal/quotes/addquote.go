package quotes

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	internalBot "github.com/graffic/wanon-go/internal/bot"
	"gorm.io/gorm"
)

// AddQuoteHandler handles the /addquote command
// This ports the Quotes.AddQuote functionality from Elixir
type AddQuoteHandler struct {
	internalBot.BaseHandler
	db      *gorm.DB
	builder *Builder
	store   *Store
}

func NewAddQuoteHandler(b internalBot.BotClient, db *gorm.DB, logger *slog.Logger) *AddQuoteHandler {
	if logger == nil {
		logger = slog.Default()
	}
	return &AddQuoteHandler{
		BaseHandler: internalBot.NewBaseHandler(b, logger),
		db:          db,
		builder: NewBuilder(db),
		store:   NewStore(db),
	}
}

// Handle processes the /addquote command
// This signature matches bot.HandlerFunc
func (h *AddQuoteHandler) Handle(ctx context.Context, _ *bot.Bot, update *models.Update) {
	msg := update.Message
	if msg == nil {
		return
	}

	chatID := msg.Chat.ID
	h.Logger.Info("executing /addquote command", "chat_id", chatID, "user_id", msg.From.ID)

	// Check if message is a reply
	if msg.ReplyToMessage == nil {
		h.Reply(ctx, chatID, "Please reply to a message to add it as a quote.")
		return
	}

	// Build the quote from cache
	replyMsg := msg.ReplyToMessage
	result, err := h.builder.BuildFrom(ctx, chatID, int64(replyMsg.ID))
	if err != nil {
		// If not in cache, try to use the reply message directly
		// This handles the case where the message is recent but cache missed
		result, err = h.buildFromReplyMessage(replyMsg)
		if err != nil {
			h.Reply(ctx, chatID, "Could not build quote. The message may be too old or not in cache.")
			return
		}
	}

	// Store the quote
	creator := extractUser(msg.From)

	quote, err := h.store.StoreFromBuild(ctx, creator, result)
	if err != nil {
		h.Logger.Error("failed to store quote", "error", err)
		return
	}

	// Send confirmation
	confirmation := fmt.Sprintf("Quote #%d added with %d entries!", quote.ID, len(quote.Entries))
	h.Reply(ctx, chatID, confirmation)
}

// buildFromReplyMessage builds a quote result from a reply message directly
// This is a fallback when the message is not in cache
func (h *AddQuoteHandler) buildFromReplyMessage(replyMsg *models.Message) (*BuildResult, error) {
	// Convert message to JSON
	msgJSON, err := json.Marshal(replyMsg)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal message: %w", err)
	}

	chatID := replyMsg.Chat.ID

	entry := CacheEntry{
		ChatID:    chatID,
		MessageID: int64(replyMsg.ID),
		Message:   msgJSON,
	}

	return &BuildResult{
		Entries: []CacheEntry{entry},
		ChatID:  chatID,
	}, nil
}

// extractUser extracts user info from models.User to map[string]interface{}
func extractUser(user *models.User) map[string]interface{} {
	if user == nil {
		return map[string]interface{}{"id": 0, "first_name": "Unknown"}
	}

	result := map[string]interface{}{
		"id":         user.ID,
		"first_name": user.FirstName,
	}
	if user.LastName != "" {
		result["last_name"] = user.LastName
	}
	if user.Username != "" {
		result["username"] = user.Username
	}
	return result
}

// Command returns the command name
func (h *AddQuoteHandler) Command() string {
	return "/addquote"
}

// Description returns the command description
func (h *AddQuoteHandler) Description() string {
	return "Add a quote by replying to a message"
}
