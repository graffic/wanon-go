package cache

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/go-telegram/bot/models"
	"github.com/graffic/wanon-go/internal/testutils"
	"github.com/stretchr/testify/suite"
)

type CacheIntegrationSuite struct {
	testutils.DBSuite
	service    *Service
	middleware *Middleware
	ctx        context.Context
}

func (s *CacheIntegrationSuite) SetupSuite() {
	s.DBSuite.SetupSuite()
	s.service = NewService(s.DBSuite.DB)
	s.middleware = NewMiddleware(s.service, slog.Default())
	s.ctx = context.Background()
}

func (s *CacheIntegrationSuite) TestHandleUpdate_AddMessage() {
	chatID := int64(-10012345678)
	msgID := int64(123)
	text := "Hello, world!"
	now := time.Now().Unix()

	update := &models.Update{
		Message: &models.Message{
			ID:   int(msgID),
			Chat: models.Chat{ID: chatID, Type: "supergroup"},
			Date: int(now),
			Text: text,
			From: &models.User{
				ID:        456,
				FirstName: "Alice",
				Username:  "alice",
			},
		},
	}

	err := s.middleware.HandleUpdate(s.ctx, update)
	s.Require().NoError(err)

	// Verify in database
	entry, err := s.service.Get(s.ctx, chatID, msgID)
	s.Require().NoError(err)
	s.Require().Equal(chatID, entry.ChatID)
	s.Require().Equal(msgID, entry.MessageID)
	s.Require().Equal(now, entry.Date)
	s.Require().Nil(entry.ReplyID)
	s.Require().Contains(string(entry.Message), text)
}

func (s *CacheIntegrationSuite) TestHandleUpdate_ChainMessage() {
	chatID := int64(-10012345678)
	now := time.Now().Unix()

	// 1. Original message
	msg1ID := int64(101)
	update1 := &models.Update{
		Message: &models.Message{
			ID:   int(msg1ID),
			Chat: models.Chat{ID: chatID, Type: "supergroup"},
			Date: int(now),
			Text: "First message",
		},
	}
	s.Require().NoError(s.middleware.HandleUpdate(s.ctx, update1))

	// 2. Reply message
	msg2ID := int64(102)
	update2 := &models.Update{
		Message: &models.Message{
			ID:   int(msg2ID),
			Chat: models.Chat{ID: chatID, Type: "supergroup"},
			Date: int(now + 1),
			Text: "Second message",
			ReplyToMessage: &models.Message{
				ID: int(msg1ID),
			},
		},
	}
	s.Require().NoError(s.middleware.HandleUpdate(s.ctx, update2))

	// Verify chain
	chain, err := s.service.GetChain(s.ctx, chatID, msg2ID)
	s.Require().NoError(err)
	s.Require().Len(chain, 2)
	s.Require().Equal(msg1ID, chain[0].MessageID)
	s.Require().Equal(msg2ID, chain[1].MessageID)
	s.Require().NotNil(chain[1].ReplyID)
	s.Require().Equal(msg1ID, *chain[1].ReplyID)
}

func (s *CacheIntegrationSuite) TestHandleUpdate_EditMessage() {
	chatID := int64(-10012345678)
	msgID := int64(201)
	now := time.Now().Unix()

	// 1. Add original message
	update1 := &models.Update{
		Message: &models.Message{
			ID:   int(msgID),
			Chat: models.Chat{ID: chatID, Type: "supergroup"},
			Date: int(now),
			Text: "Original text",
		},
	}
	s.Require().NoError(s.middleware.HandleUpdate(s.ctx, update1))

	// 2. Edit the message
	newText := "Edited text"
	editDate := now + 10
	update2 := &models.Update{
		EditedMessage: &models.Message{
			ID:       int(msgID),
			Chat:     models.Chat{ID: chatID, Type: "supergroup"},
			Date:     int(now),
			EditDate: int(editDate),
			Text:     newText,
		},
	}
	s.Require().NoError(s.middleware.HandleUpdate(s.ctx, update2))

	// Verify update in database
	entry, err := s.service.Get(s.ctx, chatID, msgID)
	s.Require().NoError(err)
	s.Require().Contains(string(entry.Message), newText)
	s.Require().NotContains(string(entry.Message), "Original text")
}

func TestCacheIntegrationSuite(t *testing.T) {
	suite.Run(t, new(CacheIntegrationSuite))
}
