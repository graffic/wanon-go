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

// MockTelegramClient is a mock for the Telegram client
type MockTelegramClient struct {
	mock.Mock
}

func (m *MockTelegramClient) SendMessage(ctx context.Context, chatID int64, text string) error {
	args := m.Called(ctx, chatID, text)
	return args.Error(0)
}



func TestAddQuoteHandler_CanHandle(t *testing.T) {
	db := testutils.NewTestDB(t)
	handler := NewAddQuoteHandler(db.DB, nil)

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
			name: "/addquote command",
			message: &TelegramMessage{
				Text: "/addquote",
			},
			expected: true,
		},
		{
			name: "/addquote with text",
			message: &TelegramMessage{
				Text: "/addquote something",
			},
			expected: true,
		},
		{
			name: "/ADDQUOTE uppercase",
			message: &TelegramMessage{
				Text: "/ADDQUOTE",
			},
			expected: true,
		},
		{
			name: "/AddQuote mixed case",
			message: &TelegramMessage{
				Text: "/AddQuote",
			},
			expected: true,
		},
		{
			name: "whitespace before command",
			message: &TelegramMessage{
				Text: "  /addquote",
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

func TestAddQuoteHandler_Handle_WithoutReply(t *testing.T) {
	db := testutils.NewTestDB(t)
	mockClient := new(MockTelegramClient)
	handler := NewAddQuoteHandler(db.DB, mockClient)

	message := &TelegramMessage{
		MessageID: 1,
		Chat:      map[string]interface{}{"id": float64(-100123)},
		From:      map[string]interface{}{"id": float64(456), "first_name": "Test"},
		Text:      "/addquote",
	}

	// Expect error message to be sent
	mockClient.On("SendMessage", mock.Anything, int64(-100123), "Please reply to a message to add it as a quote.").Return(nil)

	err := handler.Handle(context.Background(), message)
	require.NoError(t, err)

	mockClient.AssertExpectations(t)
}

func TestAddQuoteHandler_Handle_WithReply_MessageInCache(t *testing.T) {
	db := testutils.NewTestDB(t)
	mockClient := new(MockTelegramClient)
	handler := NewAddQuoteHandler(db.DB, mockClient)

	// Add message to cache
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

	// Addquote command replying to the message
	message := &TelegramMessage{
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

	// Expect success message
	mockClient.On("SendMessage", mock.Anything, int64(-100123), mock.MatchedBy(func(text string) bool {
		return assert.Contains(t, text, "Quote #")
		return assert.Contains(t, text, "added with 1 entries!")
	})).Return(nil)

	err := handler.Handle(context.Background(), message)
	require.NoError(t, err)

	mockClient.AssertExpectations(t)

	// Verify quote was stored
	var quotes []Quote
	err = db.DB.Where("chat_id = ?", -100123).Find(&quotes).Error
	require.NoError(t, err)
	assert.Len(t, quotes, 1)

	// Verify entries were stored
	var entries []QuoteEntry
	err = db.DB.Where("quote_id = ?", quotes[0].ID).Find(&entries).Error
	require.NoError(t, err)
	assert.Len(t, entries, 1)
}

func TestAddQuoteHandler_Handle_WithReply_MessageNotInCache(t *testing.T) {
	db := testutils.NewTestDB(t)
	mockClient := new(MockTelegramClient)
	handler := NewAddQuoteHandler(db.DB, mockClient)

	// Addquote command replying to a message not in cache
	message := &TelegramMessage{
		MessageID: 10,
		Chat:      map[string]interface{}{"id": float64(-100123)},
		From:      map[string]interface{}{"id": float64(456), "first_name": "Test"},
		Text:      "/addquote",
		ReplyToMessage: &TelegramMessage{
			MessageID: 99,
			Chat:      map[string]interface{}{"id": float64(-100123)},
			Text:      "Direct message to quote",
			From:      map[string]interface{}{"id": float64(789), "first_name": "Original"},
		},
	}

	// Expect success message
	mockClient.On("SendMessage", mock.Anything, int64(-100123), mock.MatchedBy(func(text string) bool {
		return assert.Contains(t, text, "Quote #")
		return assert.Contains(t, text, "added with 1 entries!")
	})).Return(nil)

	err := handler.Handle(context.Background(), message)
	require.NoError(t, err)

	mockClient.AssertExpectations(t)

	// Verify quote was stored
	var quotes []Quote
	err = db.DB.Where("chat_id = ?", -100123).Find(&quotes).Error
	require.NoError(t, err)
	assert.Len(t, quotes, 1)
}

func TestAddQuoteHandler_Handle_NoChatID(t *testing.T) {
	db := testutils.NewTestDB(t)
	mockClient := new(MockTelegramClient)
	handler := NewAddQuoteHandler(db.DB, mockClient)

	message := &TelegramMessage{
		MessageID: 1,
		Chat:      nil, // No chat
		Text:      "/addquote",
		ReplyToMessage: &TelegramMessage{
			MessageID: 5,
			Text:      "Message to quote",
		},
	}

	err := handler.Handle(context.Background(), message)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "could not extract chat ID")
}

func TestAddQuoteHandler_Command(t *testing.T) {
	db := testutils.NewTestDB(t)
	handler := NewAddQuoteHandler(db.DB, nil)

	assert.Equal(t, "/addquote", handler.Command())
}

func TestAddQuoteHandler_Description(t *testing.T) {
	db := testutils.NewTestDB(t)
	handler := NewAddQuoteHandler(db.DB, nil)

	assert.Equal(t, "Add a quote by replying to a message", handler.Description())
}
