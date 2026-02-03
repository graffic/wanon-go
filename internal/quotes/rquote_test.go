package quotes

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/graffic/wanon-go/internal/testutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
)

func TestRQuoteHandler_Command(t *testing.T) {
	db := testutils.NewTestDB(t)
	handler := NewRQuoteHandler(db.DB)

	assert.Equal(t, "/rquote", handler.Command())
}

func TestRQuoteHandler_Description(t *testing.T) {
	db := testutils.NewTestDB(t)
	handler := NewRQuoteHandler(db.DB)

	assert.Equal(t, "Get a random quote from this chat", handler.Description())
}

func TestRQuoteHandler_Handle_NoQuotes(t *testing.T) {
	db := testutils.NewTestDB(t)
	handler := NewRQuoteHandler(db.DB)

	// Test that CountForChat returns 0 for empty chat
	count, err := handler.store.CountForChat(context.Background(), -100123)
	require.NoError(t, err)
	assert.Equal(t, int64(0), count)
}

func TestRQuoteHandler_Handle_OneQuote(t *testing.T) {
	db := testutils.NewTestDB(t)
	handler := NewRQuoteHandler(db.DB)

	// Create a quote
	creator := map[string]interface{}{"id": 123, "first_name": "Creator"}
	creatorJSON, _ := json.Marshal(creator)

	message := map[string]interface{}{
		"message_id": float64(1),
		"from":       map[string]interface{}{"first_name": "Author"},
		"date":       float64(1609459100),
		"text":       "This is a quote",
	}
	messageJSON, _ := json.Marshal(message)

	quote := Quote{
		Creator: datatypes.JSON(creatorJSON),
		ChatID:  -100123,
		Entries: []QuoteEntry{
			{Order: 0, Message: datatypes.JSON(messageJSON)},
		},
	}
	require.NoError(t, db.DB.Create(&quote).Error)

	// Test that CountForChat returns 1
	count, err := handler.store.CountForChat(context.Background(), -100123)
	require.NoError(t, err)
	assert.Equal(t, int64(1), count)

	// Test that GetRandomForChat returns the quote
	randomQuote, err := handler.store.GetRandomForChat(context.Background(), -100123)
	require.NoError(t, err)
	require.NotNil(t, randomQuote)
	assert.Equal(t, quote.ID, randomQuote.ID)

	// Test rendering
	rendered, err := handler.renderer.RenderWithDate(randomQuote)
	require.NoError(t, err)
	assert.Contains(t, rendered, "Author: This is a quote")
	assert.Contains(t, rendered, "#1")
}

func TestRQuoteHandler_Handle_MultipleQuotes(t *testing.T) {
	db := testutils.NewTestDB(t)
	handler := NewRQuoteHandler(db.DB)

	// Create multiple quotes
	creator := map[string]interface{}{"id": 123, "first_name": "Creator"}
	creatorJSON, _ := json.Marshal(creator)

	for i := 0; i < 3; i++ {
		message := map[string]interface{}{
			"message_id": float64(i + 1),
			"from":       map[string]interface{}{"first_name": "Author"},
			"date":       float64(1609459100 + int64(i)),
			"text":       "Quote",
		}
		messageJSON, _ := json.Marshal(message)

		quote := Quote{
			Creator: datatypes.JSON(creatorJSON),
			ChatID:  -100123,
			Entries: []QuoteEntry{
				{Order: 0, Message: datatypes.JSON(messageJSON)},
			},
		}
		require.NoError(t, db.DB.Create(&quote).Error)
	}

	// Test that CountForChat returns 3
	count, err := handler.store.CountForChat(context.Background(), -100123)
	require.NoError(t, err)
	assert.Equal(t, int64(3), count)

	// Test that GetRandomForChat returns a quote (any of the 3)
	randomQuote, err := handler.store.GetRandomForChat(context.Background(), -100123)
	require.NoError(t, err)
	require.NotNil(t, randomQuote)
	assert.True(t, randomQuote.ID > 0)
}

func TestRQuoteHandler_Handle_DifferentChat(t *testing.T) {
	db := testutils.NewTestDB(t)
	handler := NewRQuoteHandler(db.DB)

	// Create quote in different chat
	creator := map[string]interface{}{"id": 123, "first_name": "Creator"}
	creatorJSON, _ := json.Marshal(creator)

	message := map[string]interface{}{
		"message_id": float64(1),
		"from":       map[string]interface{}{"first_name": "Author"},
		"date":       float64(1609459100),
		"text":       "This is a quote",
	}
	messageJSON, _ := json.Marshal(message)

	quote := Quote{
		Creator: datatypes.JSON(creatorJSON),
		ChatID:  -100999, // Different chat
		Entries: []QuoteEntry{
			{Order: 0, Message: datatypes.JSON(messageJSON)},
		},
	}
	require.NoError(t, db.DB.Create(&quote).Error)

	// Test that CountForChat returns 0 for different chat
	count, err := handler.store.CountForChat(context.Background(), -100123)
	require.NoError(t, err)
	assert.Equal(t, int64(0), count)

	// Test that GetRandomForChat returns nil for different chat
	randomQuote, err := handler.store.GetRandomForChat(context.Background(), -100123)
	require.NoError(t, err)
	assert.Nil(t, randomQuote)
}
