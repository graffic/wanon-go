package stats

import (
	"context"
	"log/slog"
	"testing"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/stretchr/testify/require"
)

func TestMiddleware_EmptyUpdate(t *testing.T) {
	calledNext := false
	nextHandler := func(ctx context.Context, b *bot.Bot, update *models.Update) {
		calledNext = true
	}

	middleware := NewMiddleware(nil, slog.Default())
	handler := middleware(nextHandler)

	handler(context.Background(), nil, &models.Update{})
	require.True(t, calledNext, "next handler should be called even for empty updates")
}
