package stats

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/graffic/wanon-go/internal/testutils"
	"github.com/stretchr/testify/suite"
)

type MiddlewareIntegrationSuite struct {
	testutils.DBSuite
	service    *Service
	middleware bot.Middleware
	ctx        context.Context
}

func (s *MiddlewareIntegrationSuite) SetupSuite() {
	s.DBSuite.SetupSuite()
	s.service = NewService(s.DBSuite.DB)
	s.middleware = NewMiddleware(s.service, slog.Default())
	s.ctx = context.Background()
}

func (s *MiddlewareIntegrationSuite) TestIntegration_MessageStats() {
	chatID := int64(-10012345678)
	userID := int64(456)
	userName := "alice"

	// Track if next handler is called
	calledNext := false
	nextHandler := func(ctx context.Context, b *bot.Bot, update *models.Update) {
		calledNext = true
	}

	handler := s.middleware(nextHandler)

	// First message
	t1 := time.Now().Add(-2 * time.Hour).Unix()
	update1 := &models.Update{
		Message: &models.Message{
			ID:   1,
			Chat: models.Chat{ID: chatID, Type: "supergroup"},
			Date: int(t1),
			From: &models.User{
				ID:        userID,
				FirstName: "Alice",
				Username:  userName,
			},
		},
	}

	handler(s.ctx, nil, update1)
	s.Require().True(calledNext, "next handler should be called")

	// Verify stats
	var stats UserStats
	err := s.DB.WithContext(s.ctx).Raw("SELECT user_id, user_name, last_message_at, total_messages FROM user_message_stats WHERE chat_id = ? AND user_id = ?", chatID, userID).Scan(&stats).Error
	s.Require().NoError(err)
	s.Require().Equal(int64(1), stats.TotalMessages)
	s.Require().Equal(time.Unix(t1, 0).UTC(), stats.LastMessageAt.UTC())

	// Reset for second message
	calledNext = false

	// Second message
	t2 := time.Now().Unix()
	update2 := &models.Update{
		Message: &models.Message{
			ID:   2,
			Chat: models.Chat{ID: chatID, Type: "supergroup"},
			Date: int(t2),
			From: &models.User{
				ID:        userID,
				FirstName: "Alice",
				Username:  userName,
			},
		},
	}

	handler(s.ctx, nil, update2)
	s.Require().True(calledNext, "next handler should be called")

	// Verify stats updated
	err = s.DB.WithContext(s.ctx).Raw("SELECT user_id, user_name, last_message_at, total_messages FROM user_message_stats WHERE chat_id = ? AND user_id = ?", chatID, userID).Scan(&stats).Error
	s.Require().NoError(err)
	s.Require().Equal(int64(2), stats.TotalMessages)
	s.Require().Equal(time.Unix(t2, 0).UTC(), stats.LastMessageAt.UTC())

	// Verify daily counts
	var hourlyCount int64
	err = s.DB.WithContext(s.ctx).Raw("SELECT SUM(message_count) FROM user_message_hourly WHERE chat_id = ? AND user_id = ?", chatID, userID).Scan(&hourlyCount).Error
	s.Require().NoError(err)
	s.Require().Equal(int64(2), hourlyCount)
	
	// Top users
	var topUsers []struct {
		UserName     string
		MessageCount int64
	}
	err = s.DB.WithContext(s.ctx).Raw("SELECT user_name, total_messages as message_count FROM user_message_stats WHERE chat_id = ? ORDER BY total_messages DESC LIMIT 10", chatID).Scan(&topUsers).Error
	s.Require().NoError(err)
	s.Require().Len(topUsers, 1)
	s.Require().Equal(userName, topUsers[0].UserName)
	s.Require().Equal(int64(2), topUsers[0].MessageCount)
}

func (s *MiddlewareIntegrationSuite) TestIntegration_ChannelPost() {
	chatID := int64(-10098765432)
	postTime := time.Now().Unix()

	calledNext := false
	nextHandler := func(ctx context.Context, b *bot.Bot, update *models.Update) {
		calledNext = true
	}

	handler := s.middleware(nextHandler)

	update := &models.Update{
		ChannelPost: &models.Message{
			ID:   10,
			Chat: models.Chat{ID: chatID, Type: "channel"},
			Date: int(postTime),
			SenderChat: &models.Chat{
				ID:    chatID,
				Title: "My Channel",
			},
		},
	}

	handler(s.ctx, nil, update)
	s.Require().True(calledNext)

	// Verify stats
	var topChannels []struct {
		UserName     string
		MessageCount int64
	}
	err := s.DB.WithContext(s.ctx).Raw("SELECT user_name, total_messages as message_count FROM user_message_stats WHERE chat_id = ? ORDER BY total_messages DESC LIMIT 10", chatID).Scan(&topChannels).Error
	s.Require().NoError(err)
	s.Require().Len(topChannels, 1)
	s.Require().Equal("My Channel", topChannels[0].UserName)
}

func TestMiddlewareIntegrationSuite(t *testing.T) {
	suite.Run(t, new(MiddlewareIntegrationSuite))
}
