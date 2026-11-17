package quotes

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/graffic/wanon-go/internal/testutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"gorm.io/datatypes"
)

type rQuoteBotClient struct {
	sentMsg *bot.SendMessageParams
}

func (b *rQuoteBotClient) SendMessage(_ context.Context, params *bot.SendMessageParams) (*models.Message, error) {
	b.sentMsg = params
	return &models.Message{}, nil
}

type RQuoteDBSuite struct {
	testutils.DBSuite
}

func (s *RQuoteDBSuite) SetupSuite() {
	s.DBSuite.SetupSuite()
}

func TestRQuoteDBSuite(t *testing.T) {
	suite.Run(t, new(RQuoteDBSuite))
}

func TestRQuoteHandler_Command(t *testing.T) {
	handler := NewRQuoteHandler(nil, nil, nil)

	assert.Equal(t, "/rquote", handler.Command())
}

func TestRQuoteHandler_Description(t *testing.T) {
	handler := NewRQuoteHandler(nil, nil, nil)

	assert.Equal(t, "Get a quote from this chat. Usage: /rquote [quote ID, #ID, or search text]", handler.Description())
}

func (s *RQuoteDBSuite) storeSearchQuote(chatID int64, text string) *Quote {
	message, err := json.Marshal(map[string]interface{}{
		"text": text,
		"from": map[string]interface{}{"first_name": "Author"},
		"date": 1609459100,
	})
	s.Require().NoError(err)

	quote, err := NewStore(s.DB).Store(context.Background(), StoreOptions{
		ChatID:  chatID,
		Creator: map[string]interface{}{"id": 123, "first_name": "Creator"},
		Entries: []CacheEntry{{Message: datatypes.JSON(message)}},
	})
	s.Require().NoError(err)
	return quote
}

func (s *RQuoteDBSuite) TestRQuoteHandler_Handle_Search() {
	chatID := int64(-100123)
	quote := s.storeSearchQuote(chatID, "A quote without numbers")
	otherChatQuote := s.storeSearchQuote(-100999, "Private quote")
	missingID := otherChatQuote.ID + 1000000

	// These text matches must never be returned for an ID lookup, even if the ID is missing.
	s.storeSearchQuote(chatID, fmt.Sprintf(
		"Numbers mentioned: %d #%d %d #%d %d #%d 0 #0 9223372036854775807 9223372036854775808 #9223372036854775808",
		quote.ID, quote.ID, otherChatQuote.ID, otherChatQuote.ID, missingID, missingID,
	))
	textQuote := s.storeSearchQuote(chatID, "Hello WORLD, this happened in 2026. #tag. 9223372036854775808 reasons.")

	renderer := NewRenderer()
	renderedQuote, err := renderer.RenderWithDate(quote)
	s.Require().NoError(err)
	renderedTextQuote, err := renderer.RenderWithDate(textQuote)
	s.Require().NoError(err)

	maxText := strings.Repeat("x", 200)
	maxUnicodeText := strings.Repeat("界", 200)
	maxID := strings.Repeat("9", 200)
	maxHashID := "#" + strings.Repeat("9", 199)
	lengthError := "Search input must be at most 200 characters."

	tests := []struct {
		name  string
		query string
		want  string
	}{
		{"number", fmt.Sprint(quote.ID), renderedQuote},
		{"hash number", fmt.Sprintf("#%d", quote.ID), renderedQuote},
		{"surrounding whitespace", fmt.Sprintf("  #%d  ", quote.ID), renderedQuote},
		{"leading zeros", fmt.Sprintf("%06d", quote.ID), renderedQuote},
		{"missing number", fmt.Sprint(missingID), fmt.Sprintf("Quote #%d not found in this chat.", missingID)},
		{"missing hash number", fmt.Sprintf("#%d", missingID), fmt.Sprintf("Quote #%d not found in this chat.", missingID)},
		{"another chat's number", fmt.Sprint(otherChatQuote.ID), fmt.Sprintf("Quote #%d not found in this chat.", otherChatQuote.ID)},
		{"another chat's hash number", fmt.Sprintf("#%d", otherChatQuote.ID), fmt.Sprintf("Quote #%d not found in this chat.", otherChatQuote.ID)},
		{"zero", "0", "Quote #0 not found in this chat."},
		{"hash zero", "#0", "Quote #0 not found in this chat."},
		{"maximum ID", "9223372036854775807", "Quote #9223372036854775807 not found in this chat."},
		{"out of range ID", "9223372036854775808", "Quote #9223372036854775808 not found in this chat."},
		{"out of range hash ID", "#9223372036854775808", "Quote #9223372036854775808 not found in this chat."},
		{"case insensitive text", "world", renderedTextQuote},
		{"text containing a number", "in 2026", renderedTextQuote},
		{"text starting with a number", "2026.", renderedTextQuote},
		{"text starting with a large number", "9223372036854775808 reasons", renderedTextQuote},
		{"non-numeric hashtag", "#tag", renderedTextQuote},
		{"no text matches", "does not exist", "No quotes found containing: does not exist"},
		{"maximum text length", maxText, "No quotes found containing: " + maxText},
		{"maximum Unicode length", maxUnicodeText, "No quotes found containing: " + maxUnicodeText},
		{"maximum ID length", maxID, "Quote #" + maxID + " not found in this chat."},
		{"maximum hash ID length", maxHashID, "Quote " + maxHashID + " not found in this chat."},
		{"too long text", maxText + "x", lengthError},
		{"too long Unicode text", maxUnicodeText + "界", lengthError},
		{"too long ID", maxID + "9", lengthError},
		{"too long hash ID", "#" + maxID, lengthError},
		{"maximum Telegram command length", strings.Repeat("9", 4088), lengthError},
	}

	for _, tt := range tests {
		s.Run(tt.name, func() {
			client := &rQuoteBotClient{}
			handler := NewRQuoteHandler(client, s.DB, nil)
			handler.Handle(context.Background(), nil, &models.Update{
				Message: &models.Message{
					Chat: models.Chat{ID: chatID},
					From: &models.User{ID: 123},
					Text: "/rquote " + tt.query,
				},
			})

			s.Require().NotNil(client.sentMsg)
			s.Assert().Equal(chatID, client.sentMsg.ChatID)
			s.Assert().Equal(tt.want, client.sentMsg.Text)
		})
	}
}

func (s *RQuoteDBSuite) TestRQuoteHandler_Handle_NoQuotes() {
	handler := NewRQuoteHandler(nil, s.DB, nil)

	// Test that CountForChat returns 0 for empty chat
	count, err := handler.store.CountForChat(context.Background(), -100123)
	s.Require().NoError(err)
	s.Assert().Equal(int64(0), count)
}

func (s *RQuoteDBSuite) TestRQuoteHandler_Handle_OneQuote() {
	handler := NewRQuoteHandler(nil, s.DB, nil)

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
	s.Require().NoError(s.DB.Create(&quote).Error)

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
	s.Assert().Contains(rendered, fmt.Sprintf("#%d", randomQuote.ID))
}

func (s *RQuoteDBSuite) TestRQuoteHandler_Handle_MultipleQuotes() {
	handler := NewRQuoteHandler(nil, s.DB, nil)

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
		s.Require().NoError(s.DB.Create(&quote).Error)
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

func (s *RQuoteDBSuite) TestRQuoteHandler_Handle_DifferentChat() {
	handler := NewRQuoteHandler(nil, s.DB, nil)

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
	s.Require().NoError(s.DB.Create(&quote).Error)

	// Test that CountForChat returns 0 for different chat
	count, err := handler.store.CountForChat(context.Background(), -100123)
	s.Require().NoError(err)
	s.Assert().Equal(int64(0), count)

	// Test that GetRandomForChat returns nil for different chat
	randomQuote, err := handler.store.GetRandomForChat(context.Background(), -100123)
	s.Require().NoError(err)
	s.Assert().Nil(randomQuote)
}
