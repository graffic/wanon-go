package stats

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/go-telegram/bot"
	"github.com/go-telegram/bot/models"
	"github.com/graffic/wanon-go/internal/testutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type mockBotClient struct {
	sentMsg *bot.SendMessageParams
}

func (m *mockBotClient) SendMessage(ctx context.Context, params *bot.SendMessageParams) (*models.Message, error) {
	m.sentMsg = params
	return &models.Message{}, nil
}

type SeenIntegrationSuite struct {
	suite.Suite
	db  *testutils.TestDB
	svc *Service
	ctx context.Context
}

func (s *SeenIntegrationSuite) SetupSuite() {
	s.db = testutils.NewTestDB(s.T())
	s.svc = NewService(s.db.DB)
	s.ctx = context.Background()
}

func (s *SeenIntegrationSuite) SetupTest() {
	tables := []string{"user_message_stats", "user_message_hourly"}
	for _, table := range tables {
		s.Require().NoError(s.db.DB.Exec(fmt.Sprintf("TRUNCATE TABLE %s CASCADE", table)).Error)
	}
}

func (s *SeenIntegrationSuite) TestHandle_HappyPath() {
	chatID := int64(-10012345678)
	userID := int64(12345)
	userName := "testuser"
	msgTime := time.Date(2026, 3, 5, 12, 0, 0, 0, time.UTC)

	// Insert data directly via s.svc.RecordMessage
	err := s.svc.RecordMessage(s.ctx, chatID, userID, userName, msgTime)
	s.Require().NoError(err)

	mb := &mockBotClient{}
	h := NewSeenHandler(mb, s.svc, nil)

	seenMsg := &models.Message{
		Chat: models.Chat{ID: chatID},
		From: &models.User{
			ID:       99999,
			Username: "otheruser",
		},
		Text: "/seen @testuser",
		Date: int(time.Now().Unix()),
	}
	update := &models.Update{Message: seenMsg}

	h.Handle(s.ctx, nil, update)

	s.Require().NotNil(mb.sentMsg, "expected a message to be sent")
	s.Require().Equal(chatID, mb.sentMsg.ChatID)

	text := mb.sentMsg.Text
	s.Require().Contains(text, "User: testuser")
	s.Require().Contains(text, "Total messages: 1")

	s.Require().Contains(text, fmt.Sprintf("Last seen: %s", msgTime.Format("2006-01-02 15:04 MST")))
}

func TestSeenHandlerCommand(t *testing.T) {
	h := NewSeenHandler(nil, nil, nil)
	assert.Equal(t, "seen", h.Command())
}

func TestSeenHandlerDescription(t *testing.T) {
	h := NewSeenHandler(nil, nil, nil)
	assert.Equal(t, "Show last seen and message stats for a user", h.Description())
}
func TestSeenHandlerHandle_NoMessage(t *testing.T) {
	h := NewSeenHandler(nil, nil, nil)
	update := &models.Update{}
	h.Handle(context.Background(), nil, update)
	// No error
}

func TestSeenHandlerHandle_NoText(t *testing.T) {
	h := NewSeenHandler(nil, nil, nil)
	update := &models.Update{
		Message: &models.Message{
			Chat: models.Chat{ID: -10012345678},
			Text: "",
		},
	}
	h.Handle(context.Background(), nil, update)
	// No error
}

func TestSeenIntegrationSuite(t *testing.T) {
	suite.Run(t, new(SeenIntegrationSuite))
}
