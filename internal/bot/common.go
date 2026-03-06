package bot

import (
	"context"
	"log/slog"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

type BotClient interface {
	SendMessage(ctx context.Context, params *bot.SendMessageParams) (*models.Message, error)
}

type BaseHandler struct {
	Bot    BotClient
	Logger *slog.Logger
}

func NewBaseHandler(bot BotClient, logger *slog.Logger) BaseHandler {
	return BaseHandler{Bot: bot, Logger: logger}
}

func (h *BaseHandler) Reply(ctx context.Context, chatID int64, text string) {
	if _, err := h.Bot.SendMessage(ctx, &bot.SendMessageParams{ChatID: chatID, Text: text}); err != nil {
		h.Logger.Error("failed to send message", "error", err)
	}
}
