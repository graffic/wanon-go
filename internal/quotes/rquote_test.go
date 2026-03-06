package quotes

import (
	"context"
	"encoding/json"

	"gorm.io/datatypes"
)

func (s *QuotesDBSuite) TestRQuoteHandler_Command() {
	handler := NewRQuoteHandler(nil, s.db.DB)

	s.Assert().Equal("/rquote", handler.Command())
}

func (s *QuotesDBSuite) TestRQuoteHandler_Description() {
	handler := NewRQuoteHandler(nil, s.db.DB)

	s.Assert().Equal("Get a random quote from this chat", handler.Description())
}

func (s *QuotesDBSuite) TestRQuoteHandler_Handle_NoQuotes() {
	handler := NewRQuoteHandler(nil, s.db.DB)

	// Test that CountForChat returns 0 for empty chat
	count, err := handler.store.CountForChat(context.Background(), -100123)
	s.Require().NoError(err)
	s.Assert().Equal(int64(0), count)
}

func (s *QuotesDBSuite) TestRQuoteHandler_Handle_OneQuote() {
	handler := NewRQuoteHandler(nil, s.db.DB)

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
	s.Require().NoError(s.db.DB.Create(&quote).Error)

	// Test that CountForChat returns 1
	count, err := handler.store.CountForChat(context.Background(), -100123)
	s.Require().NoError(err)
	s.Assert().Equal(int64(1), count)

	// Test that GetRandomForChat returns the quote
	randomQuote, err := handler.store.GetRandomForChat(context.Background(), -100123)
	s.Require().NoError(err)
	s.Require().NotNil(randomQuote)
	s.Assert().Equal(quote.ID, randomQuote.ID)

	// Test rendering
	rendered, err := handler.renderer.RenderWithDate(randomQuote)
	s.Require().NoError(err)
	s.Assert().Contains(rendered, "Author: This is a quote")
	s.Assert().Contains(rendered, "#1")
}

func (s *QuotesDBSuite) TestRQuoteHandler_Handle_MultipleQuotes() {
	handler := NewRQuoteHandler(nil, s.db.DB)

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
		s.Require().NoError(s.db.DB.Create(&quote).Error)
	}

	// Test that CountForChat returns 3
	count, err := handler.store.CountForChat(context.Background(), -100123)
	s.Require().NoError(err)
	s.Assert().Equal(int64(3), count)

	// Test that GetRandomForChat returns a quote (any of the 3)
	randomQuote, err := handler.store.GetRandomForChat(context.Background(), -100123)
	s.Require().NoError(err)
	s.Require().NotNil(randomQuote)
	s.Assert().True(randomQuote.ID > 0)
}

func (s *QuotesDBSuite) TestRQuoteHandler_Handle_DifferentChat() {
	handler := NewRQuoteHandler(nil, s.db.DB)

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
	s.Require().NoError(s.db.DB.Create(&quote).Error)

	// Test that CountForChat returns 0 for different chat
	count, err := handler.store.CountForChat(context.Background(), -100123)
	s.Require().NoError(err)
	s.Assert().Equal(int64(0), count)

	// Test that GetRandomForChat returns nil for different chat
	randomQuote, err := handler.store.GetRandomForChat(context.Background(), -100123)
	s.Require().NoError(err)
	s.Assert().Nil(randomQuote)
}
