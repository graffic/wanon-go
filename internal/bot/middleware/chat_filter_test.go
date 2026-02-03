package middleware

import (
	"context"
	"log/slog"
	"testing"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
)

func TestChatFilter_AllowedChat(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(nil, nil))
	allowedChatIDs := []int64{123456789, -1009876543210}

	middleware := ChatFilter(allowedChatIDs, logger)

	called := false
	next := func(ctx context.Context, b *bot.Bot, update *models.Update) {
		called = true
	}

	update := &models.Update{
		Message: &models.Message{
			Chat: models.Chat{
				ID: 123456789,
			},
		},
	}

	handler := middleware(next)
	handler(context.Background(), nil, update)

	if !called {
		t.Error("expected handler to be called for allowed chat")
	}
}

func TestChatFilter_DeniedChat(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(nil, nil))
	allowedChatIDs := []int64{123456789}

	middleware := ChatFilter(allowedChatIDs, logger)

	called := false
	next := func(ctx context.Context, b *bot.Bot, update *models.Update) {
		called = true
	}

	update := &models.Update{
		Message: &models.Message{
			Chat: models.Chat{
				ID: 999999999,
			},
		},
	}

	handler := middleware(next)
	handler(context.Background(), nil, update)

	if called {
		t.Error("expected handler NOT to be called for denied chat")
	}
}

func TestChatFilter_AllowAll(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(nil, nil))
	// Empty allowedChatIDs means allow all
	allowedChatIDs := []int64{}

	middleware := ChatFilter(allowedChatIDs, logger)

	called := false
	next := func(ctx context.Context, b *bot.Bot, update *models.Update) {
		called = true
	}

	update := &models.Update{
		Message: &models.Message{
			Chat: models.Chat{
				ID: 999999999,
			},
		},
	}

	handler := middleware(next)
	handler(context.Background(), nil, update)

	if !called {
		t.Error("expected handler to be called when allowAll is true")
	}
}

func TestChatFilter_NilUpdate(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(nil, nil))
	allowedChatIDs := []int64{123456789}

	middleware := ChatFilter(allowedChatIDs, logger)

	called := false
	next := func(ctx context.Context, b *bot.Bot, update *models.Update) {
		called = true
	}

	handler := middleware(next)
	handler(context.Background(), nil, nil)

	if called {
		t.Error("expected handler NOT to be called for nil update")
	}
}

func TestChatFilter_NoChatID(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(nil, nil))
	allowedChatIDs := []int64{123456789}

	middleware := ChatFilter(allowedChatIDs, logger)

	called := false
	next := func(ctx context.Context, b *bot.Bot, update *models.Update) {
		called = true
	}

	// Update with no message or other chat-containing fields
	update := &models.Update{
		ID: 12345,
	}

	handler := middleware(next)
	handler(context.Background(), nil, update)

	if called {
		t.Error("expected handler NOT to be called when no chat ID is present")
	}
}

func TestChatFilter_EditedMessage(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(nil, nil))
	allowedChatIDs := []int64{123456789}

	middleware := ChatFilter(allowedChatIDs, logger)

	called := false
	next := func(ctx context.Context, b *bot.Bot, update *models.Update) {
		called = true
	}

	update := &models.Update{
		EditedMessage: &models.Message{
			Chat: models.Chat{
				ID: 123456789,
			},
		},
	}

	handler := middleware(next)
	handler(context.Background(), nil, update)

	if !called {
		t.Error("expected handler to be called for edited message in allowed chat")
	}
}

func TestChatFilter_ChannelPost(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(nil, nil))
	allowedChatIDs := []int64{-1009876543210}

	middleware := ChatFilter(allowedChatIDs, logger)

	called := false
	next := func(ctx context.Context, b *bot.Bot, update *models.Update) {
		called = true
	}

	update := &models.Update{
		ChannelPost: &models.Message{
			Chat: models.Chat{
				ID: -1009876543210,
			},
		},
	}

	handler := middleware(next)
	handler(context.Background(), nil, update)

	if !called {
		t.Error("expected handler to be called for channel post in allowed chat")
	}
}

func TestChatFilter_CallbackQuery(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(nil, nil))
	allowedChatIDs := []int64{123456789}

	middleware := ChatFilter(allowedChatIDs, logger)

	called := false
	next := func(ctx context.Context, b *bot.Bot, update *models.Update) {
		called = true
	}

	update := &models.Update{
		CallbackQuery: &models.CallbackQuery{
			Message: models.MaybeInaccessibleMessage{
				Type: models.MaybeInaccessibleMessageTypeMessage,
				Message: &models.Message{
					Chat: models.Chat{
						ID: 123456789,
					},
				},
			},
		},
	}

	handler := middleware(next)
	handler(context.Background(), nil, update)

	if !called {
		t.Error("expected handler to be called for callback query in allowed chat")
	}
}

func TestChatFilter_CallbackQueryNoMessage(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(nil, nil))
	allowedChatIDs := []int64{123456789}

	middleware := ChatFilter(allowedChatIDs, logger)

	called := false
	next := func(ctx context.Context, b *bot.Bot, update *models.Update) {
		called = true
	}

	// Callback query without a message (inline query result)
	update := &models.Update{
		CallbackQuery: &models.CallbackQuery{
			InlineMessageID: "inline123",
		},
	}

	handler := middleware(next)
	handler(context.Background(), nil, update)

	if called {
		t.Error("expected handler NOT to be called for callback query without message")
	}
}

func TestChatFilter_ChatMemberUpdate(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(nil, nil))
	allowedChatIDs := []int64{-1009876543210}

	middleware := ChatFilter(allowedChatIDs, logger)

	called := false
	next := func(ctx context.Context, b *bot.Bot, update *models.Update) {
		called = true
	}

	update := &models.Update{
		MyChatMember: &models.ChatMemberUpdated{
			Chat: models.Chat{
				ID: -1009876543210,
			},
		},
	}

	handler := middleware(next)
	handler(context.Background(), nil, update)

	if !called {
		t.Error("expected handler to be called for chat member update in allowed chat")
	}
}

func TestChatFilter_ChatJoinRequest(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(nil, nil))
	allowedChatIDs := []int64{-1009876543210}

	middleware := ChatFilter(allowedChatIDs, logger)

	called := false
	next := func(ctx context.Context, b *bot.Bot, update *models.Update) {
		called = true
	}

	update := &models.Update{
		ChatJoinRequest: &models.ChatJoinRequest{
			Chat: models.Chat{
				ID: -1009876543210,
			},
		},
	}

	handler := middleware(next)
	handler(context.Background(), nil, update)

	if !called {
		t.Error("expected handler to be called for chat join request in allowed chat")
	}
}

func TestChatFilter_MessageReaction(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(nil, nil))
	allowedChatIDs := []int64{123456789}

	middleware := ChatFilter(allowedChatIDs, logger)

	called := false
	next := func(ctx context.Context, b *bot.Bot, update *models.Update) {
		called = true
	}

	update := &models.Update{
		MessageReaction: &models.MessageReactionUpdated{
			Chat: models.Chat{
				ID: 123456789,
			},
		},
	}

	handler := middleware(next)
	handler(context.Background(), nil, update)

	if !called {
		t.Error("expected handler to be called for message reaction in allowed chat")
	}
}
