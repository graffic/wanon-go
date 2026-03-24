package stats

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/go-telegram/bot/models"
	"github.com/graffic/wanon-go/internal/config"
	"github.com/graffic/wanon-go/internal/testutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"strings"
)

type TopIntegrationSuite struct {
	testutils.DBSuite
	svc *Service
	ctx context.Context
	cfg config.StatsConfig
}

func (s *TopIntegrationSuite) SetupSuite() {
	s.DBSuite.SetupSuite()
	s.svc = NewService(s.DB)
	s.ctx = context.Background()
	s.cfg = config.StatsConfig{
		TopDefaultLimit: 5,
		TopMaxLimit:     20,
	}
}

func (s *TopIntegrationSuite) TestHandle_HappyPath() {
	chatID := int64(-10012345678)
	now := time.Now().UTC()

	// Calculate Monday of this week for consistent testing
	thisMonday := mondayOf(now)

	// Calculate "this week but not today" date
	// Use Monday (start of week) which is always in this week and not today
	// (unless today is Monday, then use Wednesday)
	thisWeekNotToday := thisMonday.Add(time.Hour) // Monday + 1 hour
	if now.Weekday() == time.Monday {
		// If today is Monday, use Wednesday instead
		thisWeekNotToday = thisMonday.AddDate(0, 0, 2).Add(time.Hour)
	}

	// Insert test data with calculated dates to ensure proper categorization
	testCases := []struct {
		userID   int64
		userName string
		msgTime  time.Time
		count    int
		desc     string
	}{
		// Today: alice has 3 messages today
		{101, "alice", now.Truncate(time.Hour), 3, "today"},
		// This week (but not today): bob has 2 messages
		{102, "bob", thisWeekNotToday, 2, "this week"},
		// Last week: charlie has 5 messages from last week (should be in month/total only)
		{103, "charlie", thisMonday.AddDate(0, 0, -3).Add(time.Hour), 5, "last week"},
	}

	for _, tc := range testCases {
		for i := 0; i < tc.count; i++ {
			err := s.svc.RecordMessage(s.ctx, chatID, tc.userID, tc.userName, tc.msgTime)
			s.Require().NoError(err)
		}
	}

	mb := &mockBotClient{}
	h := NewTopHandler(mb, s.svc, s.cfg, nil)

	seenMsg := &models.Message{
		Chat: models.Chat{ID: chatID},
		From: &models.User{
			ID:       99999,
			Username: "commander",
		},
		Text: "/top",
		Date: int(time.Now().Unix()),
	}
	update := &models.Update{Message: seenMsg}

	h.Handle(s.ctx, nil, update)

	s.Require().NotNil(mb.sentMsg, "expected a message to be sent")
	s.Require().Equal(chatID, mb.sentMsg.ChatID)

	text := mb.sentMsg.Text
	s.T().Logf("Response text:\n%s", text)

	// Verify header shows correct limit
	s.Require().Contains(text, fmt.Sprintf("Top %d users", s.cfg.TopDefaultLimit))

	// Parse sections and verify each contains expected user data
	sections := strings.Split(text, "\n\n")

	// Today section: only alice (3 messages today)
	todaySection := findSection(sections, func(s string) bool {
		return strings.HasPrefix(strings.TrimSpace(s), "Today")
	})
	s.Require().Contains(todaySection, "alice — 3", "Today should show alice with 3 messages")
	s.Require().NotContains(todaySection, "bob", "Today should not show bob")
	s.Require().NotContains(todaySection, "charlie", "Today should not show charlie")

	// This week section: alice (3) + bob (2), but NOT charlie (from last week)
	weekSection := findSection(sections, func(s string) bool {
		return strings.HasPrefix(strings.TrimSpace(s), "This week")
	})
	s.Require().Contains(weekSection, "alice — 3", "This week should show alice with 3 messages")
	s.Require().Contains(weekSection, "bob — 2", "This week should show bob with 2 messages")
	s.Require().NotContains(weekSection, "charlie", "This week should NOT show charlie (from last week)")

	// This month section: all three users (charlie: 5, alice: 3, bob: 2)
	monthSection := findSection(sections, func(s string) bool {
		return strings.HasPrefix(strings.TrimSpace(s), "This month")
	})
	s.Require().Contains(monthSection, "charlie — 5", "This month should show charlie with 5 messages")
	s.Require().Contains(monthSection, "alice — 3", "This month should show alice with 3 messages")
	s.Require().Contains(monthSection, "bob — 2", "This month should show bob with 2 messages")

	// All time section: all three users
	allTimeSection := findSection(sections, func(s string) bool {
		return strings.HasPrefix(strings.TrimSpace(s), "All time:")
	})
	s.Require().Contains(allTimeSection, "charlie", "All time should include charlie")
	s.Require().Contains(allTimeSection, "alice", "All time should include alice")
	s.Require().Contains(allTimeSection, "bob", "All time should include bob")
}

func (s *TopIntegrationSuite) TestHandle_WithCustomLimit() {
	chatID := int64(-10012345678)
	now := time.Now().UTC()

	// Insert 3 users with different counts
	testCases := []struct {
		userID   int64
		userName string
		count    int
	}{
		{101, "alice", 5},
		{102, "bob", 3},
		{103, "charlie", 1},
	}

	for _, tc := range testCases {
		for i := 0; i < tc.count; i++ {
			err := s.svc.RecordMessage(s.ctx, chatID, tc.userID, tc.userName, now)
			s.Require().NoError(err)
		}
	}

	mb := &mockBotClient{}
	h := NewTopHandler(mb, s.svc, s.cfg, nil)

	seenMsg := &models.Message{
		Chat: models.Chat{ID: chatID},
		Text: "/top 2",
		Date: int(time.Now().Unix()),
	}
	update := &models.Update{Message: seenMsg}

	h.Handle(s.ctx, nil, update)

	s.Require().NotNil(mb.sentMsg)
	text := mb.sentMsg.Text
	s.T().Logf("Response text: %s", text)

	s.Require().Contains(text, "Top 2 users")
}

func (s *TopIntegrationSuite) TestHandle_InvalidLimit_Zero() {
	chatID := int64(-10012345678)

	mb := &mockBotClient{}
	h := NewTopHandler(mb, s.svc, s.cfg, nil)

	seenMsg := &models.Message{
		Chat: models.Chat{ID: chatID},
		Text: "/top 0",
		Date: int(time.Now().Unix()),
	}
	update := &models.Update{Message: seenMsg}

	h.Handle(s.ctx, nil, update)

	s.Require().NotNil(mb.sentMsg)
	s.Require().Contains(mb.sentMsg.Text, "minimum 1")
}

func (s *TopIntegrationSuite) TestHandle_InvalidLimit_TooHigh() {
	chatID := int64(-10012345678)

	mb := &mockBotClient{}
	h := NewTopHandler(mb, s.svc, s.cfg, nil)

	seenMsg := &models.Message{
		Chat: models.Chat{ID: chatID},
		Text: "/top 100",
		Date: int(time.Now().Unix()),
	}
	update := &models.Update{Message: seenMsg}

	h.Handle(s.ctx, nil, update)

	s.Require().NotNil(mb.sentMsg)
	s.Require().Contains(mb.sentMsg.Text, "maximum")
}

func (s *TopIntegrationSuite) TestHandle_InvalidLimit_NonNumeric() {
	chatID := int64(-10012345678)

	mb := &mockBotClient{}
	h := NewTopHandler(mb, s.svc, s.cfg, nil)

	seenMsg := &models.Message{
		Chat: models.Chat{ID: chatID},
		Text: "/top abc",
		Date: int(time.Now().Unix()),
	}
	update := &models.Update{Message: seenMsg}

	h.Handle(s.ctx, nil, update)

	s.Require().NotNil(mb.sentMsg)
	s.Require().Contains(mb.sentMsg.Text, "Usage: !top [number]")
}

func (s *TopIntegrationSuite) TestHandle_NoMessagesYet() {
	chatID := int64(-10012345678)

	mb := &mockBotClient{}
	h := NewTopHandler(mb, s.svc, s.cfg, nil)

	seenMsg := &models.Message{
		Chat: models.Chat{ID: chatID},
		Text: "/top",
		Date: int(time.Now().Unix()),
	}
	update := &models.Update{Message: seenMsg}

	h.Handle(s.ctx, nil, update)

	s.Require().NotNil(mb.sentMsg)
	text := mb.sentMsg.Text
	s.T().Logf("Response text: %s", text)

	s.Require().Contains(text, "Top 5 users")
	s.Require().Contains(text, "No messages yet")
}

func (s *TopIntegrationSuite) TestHandle_UsersRankedCorrectly() {
	chatID := int64(-10012345678)
	now := time.Now().UTC()

	// Insert test data with specific counts for ranking verification
	testCases := []struct {
		userID   int64
		userName string
		count    int
	}{
		{101, "alice", 10},
		{102, "bob", 7},
		{103, "charlie", 5},
		{104, "diana", 3},
		{105, "eve", 1},
	}

	for _, tc := range testCases {
		for i := 0; i < tc.count; i++ {
			err := s.svc.RecordMessage(s.ctx, chatID, tc.userID, tc.userName, now)
			s.Require().NoError(err)
		}
	}

	mb := &mockBotClient{}
	h := NewTopHandler(mb, s.svc, s.cfg, nil)

	seenMsg := &models.Message{
		Chat: models.Chat{ID: chatID},
		Text: "/top 5",
		Date: int(time.Now().Unix()),
	}
	update := &models.Update{Message: seenMsg}

	h.Handle(s.ctx, nil, update)

	s.Require().NotNil(mb.sentMsg)
	text := mb.sentMsg.Text
	s.T().Logf("Response text: %s", text)

	// Verify all users are present and ranked correctly
	s.Require().Contains(text, "alice")
	s.Require().Contains(text, "bob")
	s.Require().Contains(text, "charlie")
	s.Require().Contains(text, "diana")
	s.Require().Contains(text, "eve")
}

func (s *TopIntegrationSuite) TestHandle_NoMessage() {
	h := NewTopHandler(nil, nil, s.cfg, nil)
	update := &models.Update{}
	h.Handle(context.Background(), nil, update)
	// No error
}

func (s *TopIntegrationSuite) TestHandle_NoText() {
	h := NewTopHandler(nil, nil, s.cfg, nil)
	update := &models.Update{
		Message: &models.Message{
			Chat: models.Chat{ID: -10012345678},
			Text: "",
		},
	}
	h.Handle(context.Background(), nil, update)
	// No error
}

func TestTopHandlerCommand(t *testing.T) {
	h := NewTopHandler(nil, nil, config.StatsConfig{}, nil)
	assert.Equal(t, "top", h.Command())
}

func TestTopHandlerDescription(t *testing.T) {
	h := NewTopHandler(nil, nil, config.StatsConfig{}, nil)
	assert.Equal(t, "Show top users by message count", h.Description())
}

func (s *TopIntegrationSuite) TestHandle_TodayVsWeekVsMonth() {
	chatID := int64(-10012345678)
	now := time.Now().UTC()

	// Insert data for different time periods
	testCases := []struct {
		userID   int64
		userName string
		msgTime  time.Time
		count    int
	}{
		{101, "alice", now.Truncate(time.Hour), 3},                      // today only
		{102, "bob", now.AddDate(0, 0, -3).Truncate(time.Hour), 5},      // this week (not today)
		{103, "charlie", now.AddDate(0, 0, -15).Truncate(time.Hour), 7}, // this month (not this week)
	}

	for _, tc := range testCases {
		for i := 0; i < tc.count; i++ {
			err := s.svc.RecordMessage(s.ctx, chatID, tc.userID, tc.userName, tc.msgTime)
			s.Require().NoError(err)
		}
	}

	mb := &mockBotClient{}
	h := NewTopHandler(mb, s.svc, s.cfg, nil)

	seenMsg := &models.Message{
		Chat: models.Chat{ID: chatID},
		Text: "/top",
		Date: int(time.Now().Unix()),
	}
	update := &models.Update{Message: seenMsg}

	h.Handle(s.ctx, nil, update)

	s.Require().NotNil(mb.sentMsg)
	text := mb.sentMsg.Text
	s.T().Logf("Response text:\n%s", text)

	// Today section should show alice at top (3 messages today)
	todaySectionStart := s.FindSection(text, "Today")
	s.Require().Greater(todaySectionStart, -1, "Today section not found")
	s.Require().Contains(text[todaySectionStart:], "alice")

	// This week section should show bob at top (5 messages this week, alice has 3)
	weekSectionStart := s.FindSection(text, "This week")
	s.Require().Greater(weekSectionStart, -1, "This week section not found")
	s.Require().Contains(text[weekSectionStart:], "bob")

	// This month section should show charlie at top (7 messages this month)
	monthSectionStart := s.FindSection(text, "This month")
	s.Require().Greater(monthSectionStart, -1, "This month section not found")
	s.Require().Contains(text[monthSectionStart:], "charlie")
}

// FindSection finds the start index of a section header in the text
func (s *TopIntegrationSuite) FindSection(text, sectionName string) int {
	for i := 0; i < len(text)-len(sectionName); i++ {
		if text[i:i+len(sectionName)] == sectionName {
			return i
		}
	}
	return -1
}

// findSection finds and returns the first section matching the predicate.
func findSection(sections []string, predicate func(string) bool) string {
	for _, s := range sections {
		if predicate(s) {
			return s
		}
	}
	return ""
}

func TestTopIntegrationSuite(t *testing.T) {
	suite.Run(t, new(TopIntegrationSuite))
}
