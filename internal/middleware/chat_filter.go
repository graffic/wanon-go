// Package middleware provides bot middleware for filtering and processing updates.
package middleware

import (
	"context"
	"log/slog"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

// ChatFilter creates a middleware that filters updates based on allowed chat IDs.
// If allowedChatIDs is empty, all chats are allowed.
// If autoLeave is true, the bot will attempt to leave unauthorized chats.
func ChatFilter(allowedChatIDs []int64, autoLeave bool, logger *slog.Logger) bot.Middleware {
	// Build lookup map for O(1) checking
	allowed := make(map[int64]bool, len(allowedChatIDs))
	for _, id := range allowedChatIDs {
		allowed[id] = true
	}
	allowAll := len(allowedChatIDs) == 0

	logger.Info("Chat filter", "allowAll", allowAll, "autoLeave", autoLeave, "chatIds", allowedChatIDs)

	return func(next bot.HandlerFunc) bot.HandlerFunc {
		return func(ctx context.Context, b *bot.Bot, update *models.Update) {
			// Extract chat ID from update
			chatID := extractChatID(update)
			if chatID == 0 {
				// No chat ID found, skip this update
				return
			}

			// Check if chat is allowed
			if !allowAll && !allowed[chatID] {
				if logger != nil {
					logger.Info("ignoring update from unauthorized chat", "chat_id", chatID)
				}

				// Attempt to leave the chat if autoLeave is enabled
				if autoLeave && b != nil {
					if logger != nil {
						logger.Info("leaving unauthorized chat", "chat_id", chatID)
					}
					_, err := b.LeaveChat(ctx, &bot.LeaveChatParams{ChatID: chatID})
					if err != nil && logger != nil {
						logger.Error("failed to leave chat", "chat_id", chatID, "error", err)
					}
				}

				return
			}

			// Chat is allowed, proceed to next handler
			next(ctx, b, update)
		}
	}
}

// extractChatID extracts the chat ID from an update.
// Returns 0 if no chat ID can be determined.
func extractChatID(update *models.Update) int64 {
	if update == nil {
		return 0
	}

	switch {
	case update.Message != nil:
		return update.Message.Chat.ID
	case update.EditedMessage != nil:
		return update.EditedMessage.Chat.ID
	case update.ChannelPost != nil:
		return update.ChannelPost.Chat.ID
	case update.EditedChannelPost != nil:
		return update.EditedChannelPost.Chat.ID
	case update.BusinessMessage != nil:
		return update.BusinessMessage.Chat.ID
	case update.EditedBusinessMessage != nil:
		return update.EditedBusinessMessage.Chat.ID
	case update.CallbackQuery != nil && update.CallbackQuery.Message.Message != nil:
		return update.CallbackQuery.Message.Message.Chat.ID
	case update.MyChatMember != nil:
		return update.MyChatMember.Chat.ID
	case update.ChatMember != nil:
		return update.ChatMember.Chat.ID
	case update.ChatJoinRequest != nil:
		return update.ChatJoinRequest.Chat.ID
	case update.MessageReaction != nil:
		return update.MessageReaction.Chat.ID
	case update.MessageReactionCount != nil:
		return update.MessageReactionCount.Chat.ID
	case update.ChatBoost != nil:
		return update.ChatBoost.Chat.ID
	case update.RemovedChatBoost != nil:
		return update.RemovedChatBoost.Chat.ID
	default:
		return 0
	}
}
