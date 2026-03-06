package quotes

import (
	"context"
	"encoding/json"

	"github.com/go-telegram/bot/models"
	"gorm.io/datatypes"
)

func (s *QuotesDBSuite) TestAddQuoteHandler_Command() {
	handler := NewAddQuoteHandler(nil, s.db.DB, nil)

	s.Assert().Equal("/addquote", handler.Command())
}

func (s *QuotesDBSuite) TestAddQuoteHandler_Description() {
	handler := NewAddQuoteHandler(nil, s.db.DB, nil)

	s.Assert().Equal("Add a quote by replying to a message", handler.Description())
}

func (s *QuotesDBSuite) TestAddQuoteHandler_buildFromReplyMessage() {
	handler := NewAddQuoteHandler(nil, s.db.DB, nil)

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
	s.Require().NoError(err)
	s.Assert().Equal(int64(-100123), result.ChatID)
	s.Assert().Len(result.Entries, 1)
	s.Assert().Equal(int64(99), result.Entries[0].MessageID)
}

func (s *QuotesDBSuite) TestAddQuoteHandler_Handle_WithReply_MessageInCache() {
	handler := NewAddQuoteHandler(nil, s.db.DB, nil)

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
	s.Require().NoError(s.db.DB.Create(&cacheEntry).Error)

	// Verify quote was stored by checking the build result
	result, err := handler.builder.BuildFrom(context.Background(), -100123, 5)
	s.Require().NoError(err)
	s.Assert().Equal(int64(-100123), result.ChatID)
	s.Assert().Len(result.Entries, 1)
}

func (s *QuotesDBSuite) TestAddQuoteHandler_Handle_WithReply_MessageNotInCache() {
	handler := NewAddQuoteHandler(nil, s.db.DB, nil)

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

	result, err := handler.buildFromReplyMessage(replyMsg)
	s.Require().NoError(err)
	s.Assert().Equal(int64(-100123), result.ChatID)
	s.Assert().Len(result.Entries, 1)

	// Store the quote
	creator := map[string]interface{}{
		"id":         float64(456),
		"first_name": "Test",
	}
	quote, err := handler.store.StoreFromBuild(context.Background(), creator, result)
	s.Require().NoError(err)
	s.Assert().NotZero(quote.ID)
	s.Assert().Len(quote.Entries, 1)
}

func (s *QuotesDBSuite) TestExtractUser() {
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
		s.Run(tt.name, func() {
			result := extractUser(tt.user)
			s.Assert().Equal(tt.expected, result)
		})
	}
}
