package stats_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/graffic/wanon-go/internal/stats"
	"github.com/graffic/wanon-go/internal/testutils"
	"github.com/stretchr/testify/suite"
)

type TopIntegrationSuite struct {
	suite.Suite
	db  *testutils.TestDB
	svc *stats.Service
	ctx context.Context
}

func (s *TopIntegrationSuite) SetupSuite() {
	s.db = testutils.NewTestDB(s.T())
	s.svc = stats.NewService(s.db.DB)
	s.ctx = context.Background()
}

func (s *TopIntegrationSuite) SetupTest() {
	tables := []string{"user_message_stats", "user_message_hourly"}
	for _, table := range tables {
		s.Require().NoError(s.db.DB.Exec(fmt.Sprintf("TRUNCATE TABLE %s CASCADE", table)).Error)
	}
}

func TestTopIntegrationSuite(t *testing.T) {
	suite.Run(t, new(TopIntegrationSuite))
}

func (s *TopIntegrationSuite) TestGetTopUsersTotal() {
	chatID := int64(-1001)
	now := time.Now().UTC()

	// Record messages for several users
	for i := 0; i < 10; i++ {
		s.Require().NoError(s.svc.RecordMessage(s.ctx, chatID, 1, "alice", now))
	}
	for i := 0; i < 5; i++ {
		s.Require().NoError(s.svc.RecordMessage(s.ctx, chatID, 2, "bob", now))
	}
	for i := 0; i < 3; i++ {
		s.Require().NoError(s.svc.RecordMessage(s.ctx, chatID, 3, "charlie", now))
	}

	result, err := s.svc.GetTopUsersTotal(s.ctx, chatID, 2)
	s.Require().NoError(err)

	s.Require().Len(result, 2)
	s.Equal("alice", result[0].UserName)
	s.Equal(int64(10), result[0].MessageCount)
	s.Equal("bob", result[1].UserName)
	s.Equal(int64(5), result[1].MessageCount)
}

func (s *TopIntegrationSuite) TestGetTopUsersSince() {
	chatID := int64(-1002)
	now := time.Now().UTC()
	yesterday := now.Add(-24 * time.Hour)

	// Record messages: alice yesterday, bob today
	for i := 0; i < 5; i++ {
		s.Require().NoError(s.svc.RecordMessage(s.ctx, chatID, 1, "alice", yesterday))
	}
	for i := 0; i < 3; i++ {
		s.Require().NoError(s.svc.RecordMessage(s.ctx, chatID, 2, "bob", now))
	}

	// Query since today start - should only include bob
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	result, err := s.svc.GetTopUsersSince(s.ctx, chatID, todayStart, 10)
	s.Require().NoError(err)

	s.Require().Len(result, 1)
	s.Equal("bob", result[0].UserName)
	s.Equal(int64(3), result[0].MessageCount)

	// Query since yesterday - should include both
	result, err = s.svc.GetTopUsersSince(s.ctx, chatID, yesterday.Truncate(time.Hour), 10)
	s.Require().NoError(err)

	s.Require().Len(result, 2)
	s.Equal("alice", result[0].UserName)
	s.Equal(int64(5), result[0].MessageCount)
}

func (s *TopIntegrationSuite) TestGetTopUsersSinceMergesNameChanges() {
	chatID := int64(-1010)
	now := time.Now().UTC()
	// Place messages in two different hourly buckets so they create separate
	// hourly rows with different user_name values for the same user_id.
	bucket1 := now.Add(-2 * time.Hour)
	bucket2 := now

	// Same user_id (42) but different names in each bucket.
	for i := 0; i < 3; i++ {
		s.Require().NoError(s.svc.RecordMessage(s.ctx, chatID, 42, "Alice Wonderland", bucket1))
	}
	for i := 0; i < 4; i++ {
		s.Require().NoError(s.svc.RecordMessage(s.ctx, chatID, 42, "alicew", bucket2))
	}

	since := bucket1.Truncate(time.Hour)
	result, err := s.svc.GetTopUsersSince(s.ctx, chatID, since, 10)
	s.Require().NoError(err)

	// The user should appear as a single entry with combined count.
	s.Require().Len(result, 1)
	s.Equal(int64(7), result[0].MessageCount)
	// Should use the most recent name.
	s.Equal("alicew", result[0].UserName)
}

func (s *TopIntegrationSuite) TestGetTopUsersSinceEmpty() {
	result, err := s.svc.GetTopUsersSince(s.ctx, -9999, time.Now().UTC(), 5)
	s.Require().NoError(err)
	s.Empty(result)
}

func (s *TopIntegrationSuite) TestGetTopUsersTotalIsolatedByChat() {
	now := time.Now().UTC()

	// Record messages in two different chats
	for i := 0; i < 5; i++ {
		s.Require().NoError(s.svc.RecordMessage(s.ctx, -1003, 1, "alice", now))
	}
	for i := 0; i < 3; i++ {
		s.Require().NoError(s.svc.RecordMessage(s.ctx, -1004, 2, "bob", now))
	}

	// Query chat -1003 should only have alice
	result, err := s.svc.GetTopUsersTotal(s.ctx, -1003, 10)
	s.Require().NoError(err)
	s.Require().Len(result, 1)
	s.Equal("alice", result[0].UserName)
}
