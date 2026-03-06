package quotes

import (
	"context"
	"encoding/json"

	"gorm.io/datatypes"
)

func (s *QuotesDBSuite) TestQuotesIntegration_AddAndRetrieve() {
	db := s.db

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
	s.Require().NoError(db.DB.Create(&cacheEntry).Error)

	// Create addquote handler
	addQuote := NewAddQuoteHandler(nil, db.DB, nil)

	// Verify the quote can be built from cache
	result, err := addQuote.builder.BuildFrom(context.Background(), -100123, 5)
	s.Require().NoError(err)
	s.Assert().Equal(int64(-100123), result.ChatID)
	s.Assert().Len(result.Entries, 1)

	// Store the quote
	creator := map[string]interface{}{
		"id":         float64(456),
		"first_name": "Test",
	}
	quote, err := addQuote.store.StoreFromBuild(context.Background(), creator, result)
	s.Require().NoError(err)
	s.Assert().NotZero(quote.ID)
	s.Assert().Len(quote.Entries, 1)

	// Create rquote handler
	rQuote := NewRQuoteHandler(nil, db.DB, nil)

	// Verify the quote can be retrieved
	randomQuote, err := rQuote.store.GetRandomForChat(context.Background(), -100123)
	s.Require().NoError(err)
	s.Require().NotNil(randomQuote)
	s.Assert().Equal(quote.ID, randomQuote.ID)

	// Verify the quote can be rendered
	rendered, err := rQuote.renderer.RenderWithDate(randomQuote)
	s.Require().NoError(err)
	s.Assert().Contains(rendered, "Original")
	s.Assert().Contains(rendered, "Message to quote")
}

func (s *QuotesDBSuite) TestQuotesIntegration_MultipleQuotes() {
	db := s.db

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
		s.Require().NoError(db.DB.Create(&quote).Error)
	}

	// Create rquote handler
	rQuote := NewRQuoteHandler(nil, db.DB, nil)

	// Verify count
	count, err := rQuote.store.CountForChat(context.Background(), -100123)
	s.Require().NoError(err)
	s.Assert().Equal(int64(3), count)

	// Request random quotes multiple times
	foundQuotes := make(map[string]bool)
	for i := 0; i < 10; i++ {
		randomQuote, err := rQuote.store.GetRandomForChat(context.Background(), -100123)
		s.Require().NoError(err)
		s.Require().NotNil(randomQuote)

		rendered, err := rQuote.renderer.RenderWithDate(randomQuote)
		s.Require().NoError(err)

		// Track which quotes we found
		for _, q := range quotes {
			if contains(rendered, q.author) && contains(rendered, q.text) {
				foundQuotes[q.text] = true
			}
		}
	}

	// We should have found at least some of the quotes
	s.Assert().GreaterOrEqual(len(foundQuotes), 1)
}

func (s *QuotesDBSuite) TestQuotesIntegration_ReplyChain() {
	db := s.db

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
	s.Require().NoError(db.DB.Create(&cacheEntry1).Error)

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
	s.Require().NoError(db.DB.Create(&cacheEntry2).Error)

	msg3JSON, _ := json.Marshal(msg3)
	replyID3 := int64(2)
	cacheEntry3 := CacheEntry{
		ChatID:    -100123,
		MessageID: 3,
		ReplyID:   &replyID3,
		Date:      1609459100,
		Message:   datatypes.JSON(msg3JSON),
	}
	s.Require().NoError(db.DB.Create(&cacheEntry3).Error)

	// Create addquote handler
	addQuote := NewAddQuoteHandler(nil, db.DB, nil)

	// Build quote from message 3 (should include chain)
	result, err := addQuote.builder.BuildFrom(context.Background(), -100123, 3)
	s.Require().NoError(err)
	s.Assert().Equal(int64(-100123), result.ChatID)
	s.Assert().Len(result.Entries, 3)
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
