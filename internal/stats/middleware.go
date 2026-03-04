package stats

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

// NewMiddleware creates a bot middleware that records user message stats.
func NewMiddleware(service *Service, logger *slog.Logger) bot.Middleware {
	if logger == nil {
		logger = slog.Default()
	}

	return func(next bot.HandlerFunc) bot.HandlerFunc {
		return func(ctx context.Context, b *bot.Bot, update *models.Update) {
			if err := recordUpdate(ctx, service, update); err != nil {
				logger.Error("stats middleware error", "error", err)
			}
			next(ctx, b, update)
		}
	}
}

func recordUpdate(ctx context.Context, service *Service, update *models.Update) error {
	if update == nil {
		return nil
	}

	msg := messageFromUpdate(update)
	if msg == nil {
		return nil
	}

	sender, ok := senderFromMessage(msg)
	if !ok {
		return nil
	}

	messageTime := time.Unix(int64(msg.Date), 0).UTC()
	return service.RecordMessage(ctx, msg.Chat.ID, sender.ID, sender.Name, messageTime)
}

func messageFromUpdate(update *models.Update) *models.Message {
	if update.Message != nil {
		return update.Message
	}
	if update.ChannelPost != nil {
		return update.ChannelPost
	}
	return nil
}

type senderInfo struct {
	ID   int64
	Name string
}

func senderFromMessage(msg *models.Message) (senderInfo, bool) {
	if msg.From != nil {
		return senderInfo{ID: msg.From.ID, Name: userNameFromUser(msg.From)}, true
	}
	if msg.SenderChat != nil {
		return senderInfo{ID: msg.SenderChat.ID, Name: chatNameFromSender(msg.SenderChat)}, true
	}
	return senderInfo{}, false
}

func userNameFromUser(user *models.User) string {
	if user == nil {
		return ""
	}
	if user.Username != "" {
		return user.Username
	}
	return strings.TrimSpace(strings.Join([]string{user.FirstName, user.LastName}, " "))
}

func chatNameFromSender(chat *models.Chat) string {
	if chat == nil {
		return ""
	}
	if chat.Username != "" {
		return chat.Username
	}
	return chat.Title
}
