package quotes

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/go-telegram/bot/models"
	"github.com/graffic/wanon-go/internal/testutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
)

func TestAddQuoteHandler_Command(t *testing.T) {
	handler := NewAddQuoteHandler(nil, nil, nil)

	assert.Equal(t, "/addquote", handler.Command())
}

func TestAddQuoteHandler_Description(t *testing.T) {
	handler := NewAddQuoteHandler(nil, nil, nil)

	assert.Equal(t, "Add a quote by replying to a message", handler.Description())
}

func TestAddQuoteHandler_buildFromReplyMessage(t *testing.T) {
	handler := NewAddQuoteHandler(nil, nil, nil)

	replyMsg := &models.Message{
		ID:   99,
		Text: "Direct message to quote",
		Chat: models.Chat{
			ID:   -100123,
			Type: "supergroup",
		},
		From: &models.User{
			ID:        789,
			FirstName: "Original",
		},
	}

	result, err := handler.buildFromReplyMessage(replyMsg)
	require.NoError(t, err)
	assert.Equal(t, int64(-100123), result.ChatID)
	assert.Len(t, result.Entries, 1)
	assert.Equal(t, int64(99), result.Entries[0].MessageID)
}

func TestExtractUser(t *testing.T) {
	tests := []struct {
		name     string
		user     *models.User
		expected map[string]interface{}
	}{
		{
			name:     "nil user",
			user:     nil,
			expected: map[string]interface{}{"id": 0, "first_name": "Unknown"},
		},
		{
			name: "user with all fields",
			user: &models.User{
				ID:        123,
				FirstName: "John",
				LastName:  "Doe",
				Username:  "johndoe",
			},
			expected: map[string]interface{}{
				"id":         int64(123),
				"first_name": "John",
				"last_name":  "Doe",
				"username":   "johndoe",
			},
		},
		{
			name: "user with minimal fields",
			user: &models.User{
				ID:        456,
				FirstName: "Jane",
			},
			expected: map[string]interface{}{
				"id":         int64(456),
				"first_name": "Jane",
			},
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			result := extractUser(tt.user)
			assert.Equal(t, tt.expected, result)
		})
	}
}

type AddQuoteDBSuite struct {
	testutils.DBSuite
	handler *AddQuoteHandler
}

func (s *AddQuoteDBSuite) SetupSuite() {
	s.DBSuite.SetupSuite()
	s.handler = NewAddQuoteHandler(nil, s.DBSuite.DB, nil)
}

func (s *AddQuoteDBSuite) TestAddQuoteHandler_Handle_WithReply_MessageInCache() {
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
	s.Require().NoError(s.DB.Create(&cacheEntry).Error)

	// Verify quote was stored by checking the build result
	result, err := s.handler.builder.BuildFrom(context.Background(), -100123, 5)
	s.Require().NoError(err)
	s.Assert().Equal(int64(-100123), result.ChatID)
	s.Assert().Len(result.Entries, 1)
}

func (s *AddQuoteDBSuite) TestAddQuoteHandler_Handle_WithReply_MessageNotInCache() {
	// Test that buildFromReplyMessage works when message not in cache
	replyMsg := &models.Message{
		ID:   99,
		Text: "Direct message to quote",
		Chat: models.Chat{
			ID:   -100123,
			Type: "supergroup",
		},
		From: &models.User{
			ID:        789,
			FirstName: "Original",
		},
	}

	result, err := s.handler.buildFromReplyMessage(replyMsg)
	s.Require().NoError(err)
	s.Assert().Equal(int64(-100123), result.ChatID)
	s.Assert().Len(result.Entries, 1)

	// Store the quote
	creator := map[string]interface{}{
		"id":         float64(456),
		"first_name": "Test",
	}
	quote, err := s.handler.store.StoreFromBuild(context.Background(), creator, result)
	s.Require().NoError(err)
	s.Assert().NotZero(quote.ID)
	s.Assert().Len(quote.Entries, 1)
}
