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

func TestQuotesIntegration_AddAndRetrieve(t *testing.T) {
	db := testutils.NewTestDB(t)

	// Setup cache with a message
	cachedMsg := map[string]interface{}{
		"message_id": float64(5),
		"chat":       map[string]interface{}{"id": float64(-100123)},
		"date":       float64(1609459100),
		"text":       "Message to quote",
		"from":       map[string]interface{}{"id": float64(789), "first_name": "Original"},
	}
	msgJSON, _ := json.Marshal(cachedMsg)
	cacheEntry := CacheEntry{
		ChatID:    -100123,
		MessageID: 5,
		Date:      1609459100,
		Message:   datatypes.JSON(msgJSON),
	}
	require.NoError(t, db.DB.Create(&cacheEntry).Error)

	// Create addquote handler
	addQuote := NewAddQuoteHandler(db.DB)

	// Verify the quote can be built from cache
	result, err := addQuote.builder.BuildFrom(context.Background(), -100123, 5)
	require.NoError(t, err)
	assert.Equal(t, int64(-100123), result.ChatID)
	assert.Len(t, result.Entries, 1)

	// Store the quote
	creator := map[string]interface{}{
		"id":         float64(456),
		"first_name": "Test",
	}
	quote, err := addQuote.store.StoreFromBuild(context.Background(), creator, result)
	require.NoError(t, err)
	assert.NotZero(t, quote.ID)
	assert.Len(t, quote.Entries, 1)

	// Create rquote handler
	rQuote := NewRQuoteHandler(db.DB)

	// Verify the quote can be retrieved
	randomQuote, err := rQuote.store.GetRandomForChat(context.Background(), -100123)
	require.NoError(t, err)
	require.NotNil(t, randomQuote)
	assert.Equal(t, quote.ID, randomQuote.ID)

	// Verify the quote can be rendered
	rendered, err := rQuote.renderer.RenderWithDate(randomQuote)
	require.NoError(t, err)
	assert.Contains(t, rendered, "Original")
	assert.Contains(t, rendered, "Message to quote")
}

func TestQuotesIntegration_MultipleQuotes(t *testing.T) {
	db := testutils.NewTestDB(t)

	// Create multiple quotes
	creator := map[string]interface{}{"id": 123, "first_name": "Creator"}
	creatorJSON, _ := json.Marshal(creator)

	quotes := []struct {
		author string
		text   string
	}{
		{"Author1", "Quote 1"},
		{"Author2", "Quote 2"},
		{"Author3", "Quote 3"},
	}

	for _, q := range quotes {
		message := map[string]interface{}{
			"message_id": float64(1),
			"from":       map[string]interface{}{"first_name": q.author},
			"date":       float64(1609459100),
			"text":       q.text,
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

	// Create rquote handler
	rQuote := NewRQuoteHandler(db.DB)

	// Verify count
	count, err := rQuote.store.CountForChat(context.Background(), -100123)
	require.NoError(t, err)
	assert.Equal(t, int64(3), count)

	// Request random quotes multiple times
	foundQuotes := make(map[string]bool)
	for i := 0; i < 10; i++ {
		randomQuote, err := rQuote.store.GetRandomForChat(context.Background(), -100123)
		require.NoError(t, err)
		require.NotNil(t, randomQuote)

		rendered, err := rQuote.renderer.RenderWithDate(randomQuote)
		require.NoError(t, err)

		// Track which quotes we found
		for _, q := range quotes {
			if contains(rendered, q.author) && contains(rendered, q.text) {
				foundQuotes[q.text] = true
			}
		}
	}

	// We should have found at least some of the quotes
	assert.GreaterOrEqual(t, len(foundQuotes), 1)
}

func TestQuotesIntegration_ReplyChain(t *testing.T) {
	db := testutils.NewTestDB(t)

	// Create a chain of messages in cache
	msg1 := map[string]interface{}{
		"message_id": float64(1),
		"chat":       map[string]interface{}{"id": float64(-100123)},
		"date":       float64(1609459000),
		"text":       "First",
		"from":       map[string]interface{}{"id": float64(1), "first_name": "User1"},
	}
	msg2 := map[string]interface{}{
		"message_id": float64(2),
		"chat":       map[string]interface{}{"id": float64(-100123)},
		"date":       float64(1609459050),
		"text":       "Second",
		"from":       map[string]interface{}{"id": float64(2), "first_name": "User2"},
	}
	msg3 := map[string]interface{}{
		"message_id": float64(3),
		"chat":       map[string]interface{}{"id": float64(-100123)},
		"date":       float64(1609459100),
		"text":       "Third",
		"from":       map[string]interface{}{"id": float64(3), "first_name": "User3"},
	}

	// msg2 replies to msg1
	msg1JSON, _ := json.Marshal(msg1)
	cacheEntry1 := CacheEntry{
		ChatID:    -100123,
		MessageID: 1,
		Date:      1609459000,
		Message:   datatypes.JSON(msg1JSON),
	}
	require.NoError(t, db.DB.Create(&cacheEntry1).Error)

	// msg3 replies to msg2
	msg2JSON, _ := json.Marshal(msg2)
	replyID2 := int64(1)
	cacheEntry2 := CacheEntry{
		ChatID:    -100123,
		MessageID: 2,
		ReplyID:   &replyID2,
		Date:      1609459050,
		Message:   datatypes.JSON(msg2JSON),
	}
	require.NoError(t, db.DB.Create(&cacheEntry2).Error)

	msg3JSON, _ := json.Marshal(msg3)
	replyID3 := int64(2)
	cacheEntry3 := CacheEntry{
		ChatID:    -100123,
		MessageID: 3,
		ReplyID:   &replyID3,
		Date:      1609459100,
		Message:   datatypes.JSON(msg3JSON),
	}
	require.NoError(t, db.DB.Create(&cacheEntry3).Error)

	// Create addquote handler
	addQuote := NewAddQuoteHandler(db.DB)

	// Build quote from message 3 (should include chain)
	result, err := addQuote.builder.BuildFrom(context.Background(), -100123, 3)
	require.NoError(t, err)
	assert.Equal(t, int64(-100123), result.ChatID)
	assert.Len(t, result.Entries, 3)
}

// Helper function
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
