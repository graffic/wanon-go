package quotes

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/graffic/wanon-go/internal/testutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
)



func TestRQuoteHandler_CanHandle(t *testing.T) {
	db := testutils.NewTestDB(t)
	handler := NewRQuoteHandler(db.DB, nil)

	tests := []struct {
		name     string
		message  *TelegramMessage
		expected bool
	}{
		{
			name:     "nil message",
			message:  nil,
			expected: false,
		},
		{
			name: "empty text",
			message: &TelegramMessage{
				Text: "",
			},
			expected: false,
		},
		{
			name: "regular message",
			message: &TelegramMessage{
				Text: "Hello world",
			},
			expected: false,
		},
		{
			name: "/rquote command",
			message: &TelegramMessage{
				Text: "/rquote",
			},
			expected: true,
		},
		{
			name: "/rquote with text",
			message: &TelegramMessage{
				Text: "/rquote something",
			},
			expected: true,
		},
		{
			name: "/RQUOTE uppercase",
			message: &TelegramMessage{
				Text: "/RQUOTE",
			},
			expected: true,
		},
		{
			name: "/RQuote mixed case",
			message: &TelegramMessage{
				Text: "/RQuote",
			},
			expected: true,
		},
		{
			name: "whitespace before command",
			message: &TelegramMessage{
				Text: "  /rquote",
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := handler.CanHandle(tt.message)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestRQuoteHandler_Handle_NoQuotes(t *testing.T) {
	db := testutils.NewTestDB(t)
	mockClient := new(MockTelegramClient)
	
	handler := NewRQuoteHandler(db.DB, mockClient)

	message := &TelegramMessage{
		MessageID: 1,
		Chat:      map[string]interface{}{"id": float64(-100123)},
		From:      map[string]interface{}{"id": float64(456), "first_name": "Test"},
		Text:      "/rquote",
	}

	// Expect message when no quotes exist
	mockClient.On("SendMessage", mock.Anything, int64(-100123), "No quotes found in this chat. Add some with /addquote!").Return(nil)

	err := handler.Handle(context.Background(), message)
	require.NoError(t, err)

	mockClient.AssertExpectations(t)
}

func TestRQuoteHandler_Handle_OneQuote(t *testing.T) {
	db := testutils.NewTestDB(t)
	mockClient := new(MockTelegramClient)
	
	handler := NewRQuoteHandler(db.DB, mockClient)

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

	messageCmd := &TelegramMessage{
		MessageID: 1,
		Chat:      map[string]interface{}{"id": float64(-100123)},
		From:      map[string]interface{}{"id": float64(456), "first_name": "Test"},
		Text:      "/rquote",
	}

	// Expect the quote to be sent (with ID and date)
	mockClient.On("SendMessage", mock.Anything, int64(-100123), mock.MatchedBy(func(text string) bool {
		return assert.Contains(t, text, "Author: This is a quote") &&
			assert.Contains(t, text, "#1")
	})).Return(nil)

	err := handler.Handle(context.Background(), messageCmd)
	require.NoError(t, err)

	mockClient.AssertExpectations(t)
}

func TestRQuoteHandler_Handle_MultipleQuotes(t *testing.T) {
	db := testutils.NewTestDB(t)
	mockClient := new(MockTelegramClient)
	
	handler := NewRQuoteHandler(db.DB, mockClient)

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

	message := &TelegramMessage{
		MessageID: 1,
		Chat:      map[string]interface{}{"id": float64(-100123)},
		From:      map[string]interface{}{"id": float64(456), "first_name": "Test"},
		Text:      "/rquote",
	}

	// Expect any one of the quotes to be sent (random selection)
	mockClient.On("SendMessage", mock.Anything, int64(-100123), mock.AnythingOfType("string")).Return(nil).Once()

	err := handler.Handle(context.Background(), message)
	require.NoError(t, err)

	mockClient.AssertExpectations(t)
}

func TestRQuoteHandler_Handle_DifferentChat(t *testing.T) {
	db := testutils.NewTestDB(t)
	mockClient := new(MockTelegramClient)
	
	handler := NewRQuoteHandler(db.DB, mockClient)

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

	messageCmd := &TelegramMessage{
		MessageID: 1,
		Chat:      map[string]interface{}{"id": float64(-100123)}, // Requesting from different chat
		From:      map[string]interface{}{"id": float64(456), "first_name": "Test"},
		Text:      "/rquote",
	}

	// Expect "no quotes" message since no quotes in this chat
	mockClient.On("SendMessage", mock.Anything, int64(-100123), "No quotes found in this chat. Add some with /addquote!").Return(nil)

	err := handler.Handle(context.Background(), messageCmd)
	require.NoError(t, err)

	mockClient.AssertExpectations(t)
}

func TestRQuoteHandler_Handle_NoChatID(t *testing.T) {
	db := testutils.NewTestDB(t)
	mockClient := new(MockTelegramClient)
	
	handler := NewRQuoteHandler(db.DB, mockClient)

	message := &TelegramMessage{
		MessageID: 1,
		Chat:      nil, // No chat
		Text:      "/rquote",
	}

	err := handler.Handle(context.Background(), message)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "could not extract chat ID")
}

func TestRQuoteHandler_Command(t *testing.T) {
	db := testutils.NewTestDB(t)
	handler := NewRQuoteHandler(db.DB, nil)

	assert.Equal(t, "/rquote", handler.Command())
}

func TestRQuoteHandler_Description(t *testing.T) {
	db := testutils.NewTestDB(t)
	handler := NewRQuoteHandler(db.DB, nil)

	assert.Equal(t, "Get a random quote from this chat", handler.Description())
}
