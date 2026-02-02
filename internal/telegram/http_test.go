package telegram

import (
	"context"
	"testing"
	"time"

	"github.com/go-telegram/bot/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHTTPClient_NewHTTPClient(t *testing.T) {
	// This test just verifies that NewHTTPClient doesn't return an error
	// with a dummy token. It won't actually connect to Telegram.
	// Note: This will fail if the token format is invalid, but go-telegram/bot
	// accepts any string as a token.
	client, err := NewHTTPClient("test-token")
	require.NoError(t, err)
	assert.NotNil(t, client)
	assert.NotNil(t, client.bot)
	assert.NotNil(t, client.updatesCh)
}

func TestHTTPClient_NewHTTPClient_WithDebug(t *testing.T) {
	client, err := NewHTTPClient("test-token", WithDebug())
	require.NoError(t, err)
	assert.NotNil(t, client)
}

func TestHTTPClient_handleUpdate(t *testing.T) {
	client, err := NewHTTPClient("test-token")
	require.NoError(t, err)

	// Create a handler to capture updates
	var receivedUpdate *models.Update
	handlerCalled := make(chan bool, 1)
	client.RegisterHandler(func(ctx context.Context, update *models.Update) {
		receivedUpdate = update
		handlerCalled <- true
	})

	update := &models.Update{
		ID: 1,
		Message: &models.Message{
			ID:   1,
			Chat: models.Chat{ID: 123, Type: "private"},
			Text: "Hello",
		},
	}

	// Call handleUpdate directly
	client.handleUpdate(context.Background(), update)

	// Wait for handler to be called
	select {
	case <-handlerCalled:
		// Handler was called
	case <-context.Background().Done():
		t.Fatal("handler was not called")
	}

	require.NotNil(t, receivedUpdate)
	assert.Equal(t, int64(1), receivedUpdate.ID)
	assert.Equal(t, "Hello", receivedUpdate.Message.Text)
}

func TestHTTPClient_RegisterHandler(t *testing.T) {
	client, err := NewHTTPClient("test-token")
	require.NoError(t, err)

	// Initially no handlers
	assert.Len(t, client.handlers, 0)

	// Register a handler
	client.RegisterHandler(func(ctx context.Context, update *models.Update) {})
	assert.Len(t, client.handlers, 1)

	// Register another handler
	client.RegisterHandler(func(ctx context.Context, update *models.Update) {})
	assert.Len(t, client.handlers, 2)
}

func TestHTTPClient_GetUpdates_Channel(t *testing.T) {
	client, err := NewHTTPClient("test-token")
	require.NoError(t, err)

	// Send an update to the channel
	expectedUpdates := []models.Update{
		{
			ID: 1,
			Message: &models.Message{
				ID:   1,
				Chat: models.Chat{ID: 123, Type: "private"},
				Text: "Hello",
			},
		},
	}

	// Send to channel in background
	go func() {
		client.updatesCh <- expectedUpdates
	}()

	// Get updates
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	updates, err := client.GetUpdates(ctx, 0, 100, 10)
	require.NoError(t, err)
	assert.Len(t, updates, 1)
	assert.Equal(t, int64(1), updates[0].ID)
}

func TestHTTPClient_GetUpdates_ContextCancellation(t *testing.T) {
	client, err := NewHTTPClient("test-token")
	require.NoError(t, err)

	// Create a cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Get updates should return context error
	_, err = client.GetUpdates(ctx, 0, 100, 10)
	assert.ErrorIs(t, err, context.Canceled)
}
