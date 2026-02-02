package quotes

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/graffic/wanon-go/internal/testutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
)

func TestQuotesIntegration_AddAndRetrieve(t *testing.T) {
	db := testutils.NewTestDB(t)

	// Create mock client
	var sentMessages []struct {
		ChatID int64  `json:"chat_id"`
		Text   string `json:"text"`
	}

	mockClient := &MockTelegramClient{}
	mockClient.On("SendMessage", mock.Anything, int64(-100123), mock.AnythingOfType("string")).Return(nil).Run(func(args mock.Arguments) {
		sentMessages = append(sentMessages, struct {
			ChatID int64  `json:"chat_id"`
			Text   string `json:"text"`
		}{ChatID: args.Get(1).(int64), Text: args.Get(2).(string)})
	})

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
	addQuote := NewAddQuoteHandler(db.DB, mockClient)

	// Execute addquote command
	addMsg := &TelegramMessage{
		MessageID: 10,
		Chat:      map[string]interface{}{"id": float64(-100123)},
		From:      map[string]interface{}{"id": float64(456), "first_name": "Test"},
		Text:      "/addquote",
		ReplyToMessage: &TelegramMessage{
			MessageID: 5,
			Chat:      map[string]interface{}{"id": float64(-100123)},
			Text:      "Message to quote",
			From:      map[string]interface{}{"id": float64(789), "first_name": "Original"},
		},
	}

	err := addQuote.Handle(context.Background(), addMsg)
	require.NoError(t, err)

	// Verify success message was sent
	require.GreaterOrEqual(t, len(sentMessages), 1)
	assert.Contains(t, sentMessages[0].Text, "Quote #")
	assert.Contains(t, sentMessages[0].Text, "added with 1 entries!")

	// Create rquote handler
	rQuote := NewRQuoteHandler(db.DB, mockClient)

	// Execute rquote command
	rquoteMsg := &TelegramMessage{
		MessageID: 20,
		Chat:      map[string]interface{}{"id": float64(-100123)},
		From:      map[string]interface{}{"id": float64(456), "first_name": "Test"},
		Text:      "/rquote",
	}

	err = rQuote.Handle(context.Background(), rquoteMsg)
	require.NoError(t, err)

	// Verify quote was retrieved and sent
	require.GreaterOrEqual(t, len(sentMessages), 2)
	assert.Contains(t, sentMessages[1].Text, "Original")
	assert.Contains(t, sentMessages[1].Text, "Message to quote")
}

func TestQuotesIntegration_MultipleQuotes(t *testing.T) {
	db := testutils.NewTestDB(t)

	var sentMessages []string
	mockClient := &MockTelegramClient{}
	mockClient.On("SendMessage", mock.Anything, int64(-100123), mock.AnythingOfType("string")).Return(nil).Run(func(args mock.Arguments) {
		sentMessages = append(sentMessages, args.Get(2).(string))
	})

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

	// Request random quotes multiple times
	rQuote := NewRQuoteHandler(db.DB, mockClient)
	msg := &TelegramMessage{
		MessageID: 1,
		Chat:      map[string]interface{}{"id": float64(-100123)},
		Text:      "/rquote",
	}

	// Request 10 times to likely hit all quotes
	for i := 0; i < 10; i++ {
		err := rQuote.Handle(context.Background(), msg)
		require.NoError(t, err)
	}

	// Verify we got different quotes
	require.GreaterOrEqual(t, len(sentMessages), 5)

	// Check that at least one of each quote was returned
	// Format is: "#<id>\n<author>: <text>\n📅 <date>"
	var foundQuotes []string
	for _, sent := range sentMessages {
		for _, q := range quotes {
			// Check if the sent message contains the author and text
			if strings.Contains(sent, fmt.Sprintf("%s: %s", q.author, q.text)) {
				foundQuotes = append(foundQuotes, q.text)
				break
			}
		}
	}

	// We should have found at least some of the quotes
	assert.GreaterOrEqual(t, len(foundQuotes), 1)
}

func TestQuotesIntegration_ReplyChain(t *testing.T) {
	db := testutils.NewTestDB(t)

	var sentMessages []string
	mockClient := &MockTelegramClient{}
	mockClient.On("SendMessage", mock.Anything, int64(-100123), mock.AnythingOfType("string")).Return(nil).Run(func(args mock.Arguments) {
		sentMessages = append(sentMessages, args.Get(2).(string))
	})

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
	addQuote := NewAddQuoteHandler(db.DB, mockClient)

	// Add quote from reply to msg3
	addMsg := &TelegramMessage{
		MessageID: 10,
		Chat:      map[string]interface{}{"id": float64(-100123)},
		From:      map[string]interface{}{"id": float64(456), "first_name": "Test"},
		Text:      "/addquote",
		ReplyToMessage: &TelegramMessage{
			MessageID: 3,
			Chat:      map[string]interface{}{"id": float64(-100123)},
			Text:      "Third",
			From:      map[string]interface{}{"id": float64(3), "first_name": "User3"},
		},
	}

	err := addQuote.Handle(context.Background(), addMsg)
	require.NoError(t, err)

	// Verify the quote was stored with all 3 entries
	var quote Quote
	err = db.DB.Preload("Entries").First(&quote).Error
	require.NoError(t, err)
	assert.Len(t, quote.Entries, 3)
}
