package stats

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/graffic/wanon-go/internal/testutils"
	"github.com/stretchr/testify/suite"
)

// ---------------------------------------------------------------------------
// Pure unit tests (no DB)
// ---------------------------------------------------------------------------

func TestParseFromID(t *testing.T) {
	tests := []struct {
		input  string
		wantID int64
		wantOK bool
	}{
		{"user502546420", 502546420, true},
		{"channel1549724404", 1549724404, true},
		{"user0", 0, true},
		{"unknown123", 0, false},
		{"", 0, false},
		{"user", 0, false},
	}
	for _, tc := range tests {
		id, ok := parseFromID(tc.input)
		if ok != tc.wantOK || id != tc.wantID {
			t.Errorf("parseFromID(%q) = (%d, %v), want (%d, %v)", tc.input, id, ok, tc.wantID, tc.wantOK)
		}
	}
}

func TestParseExportTime_UnixPreferred(t *testing.T) {
	m := &exportMessage{
		ID:           1,
		Type:         "message",
		DateUnixtime: "1660991507",
	}
	got, err := parseExportTime(m)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := time.Unix(1660991507, 0).UTC()
	if !got.Equal(want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

// ---------------------------------------------------------------------------
// DB integration suite
// ---------------------------------------------------------------------------

type ImporterDBSuite struct {
	suite.Suite
	db      *testutils.TestDB
	service *Service
}

func (s *ImporterDBSuite) SetupSuite() {
	s.db = testutils.NewTestDB(s.T())
	s.service = NewService(s.db.DB)
}

func (s *ImporterDBSuite) SetupTest() {
	tables := []string{"user_message_hourly", "user_message_stats"}
	for _, table := range tables {
		s.Require().NoError(s.db.DB.Exec(fmt.Sprintf("TRUNCATE TABLE %s CASCADE", table)).Error)
	}
}

func TestImporterDBSuite(t *testing.T) {
	suite.Run(t, new(ImporterDBSuite))
}

// minimalExport builds a JSON export string for the given messages.
func minimalExport(chatID int64, msgs []exportMessage) string {
	parts := make([]string, 0, len(msgs))
	for _, m := range msgs {
		parts = append(parts, fmt.Sprintf(
			`{"id":%d,"type":%q,"date_unixtime":%q,"from":%q,"from_id":%q}`,
			m.ID, m.Type, m.DateUnixtime, m.From, m.FromID,
		))
	}
	return fmt.Sprintf(`{"id":%d,"messages":[%s]}`, chatID, strings.Join(parts, ","))
}

// TestImportBasic verifies that a simple set of messages is imported and
// totals are computed correctly.
func (s *ImporterDBSuite) TestImportBasic() {
	ctx := context.Background()
	const chatID = int64(-1001549724404)

	// Timestamps (all CEST = UTC+2):
	//   msg1 alice  12:10 CEST = 10:10 UTC  → bucket 10:00 UTC  (epoch 1660990200)
	//   msg2 alice  12:30 CEST = 10:30 UTC  → bucket 10:00 UTC  (epoch 1660991400)
	//   msg3 bob    13:05 CEST = 11:05 UTC  → bucket 11:00 UTC  (epoch 1660993500)
	//   msg4 bob    17:00 CEST = 15:00 UTC  → bucket 15:00 UTC  (epoch 1661007600) MAX
	//
	// exportCutoff: max=17:00 CEST → truncate(17:00 CEST) = 17:00 CEST = 15:00 UTC.
	// All four message buckets ≤ 15:00 UTC → all four included.
	export := minimalExport(chatID, []exportMessage{
		{ID: 1, Type: "message", DateUnixtime: "1660990200", From: "alice", FromID: "user100"},
		{ID: 2, Type: "message", DateUnixtime: "1660991400", From: "alice", FromID: "user100"},
		{ID: 3, Type: "message", DateUnixtime: "1660993500", From: "bob", FromID: "user200"},
		{ID: 4, Type: "message", DateUnixtime: "1661007600", From: "bob", FromID: "user200"},
	})

	result, err := s.service.ImportFromExport(ctx, chatID, strings.NewReader(export))
	s.Require().NoError(err)
	s.Require().NotNil(result)
	s.Assert().Equal(4, result.MessagesProcessed)

	// Verify user_message_stats totals.
	type row struct {
		UserID        int64
		TotalMessages int64
	}
	var rows []row
	s.Require().NoError(
		s.db.DB.Raw(`SELECT user_id, total_messages FROM user_message_stats WHERE chat_id = ? ORDER BY user_id`, chatID).
			Scan(&rows).Error,
	)
	s.Require().Len(rows, 2)
	s.Assert().Equal(int64(100), rows[0].UserID)
	s.Assert().Equal(int64(2), rows[0].TotalMessages)
	s.Assert().Equal(int64(200), rows[1].UserID)
	s.Assert().Equal(int64(2), rows[1].TotalMessages)
}

// TestImportOverwrite verifies that re-importing overwrites existing hourly
// buckets and recomputes totals rather than double-counting.
func (s *ImporterDBSuite) TestImportOverwrite() {
	ctx := context.Background()
	const chatID = int64(-1001549724404)

	// Seed existing hourly data: alice has 5 messages in bucket 10:00 UTC.
	// epoch 1660989600 = 2022-08-20T10:00:00 UTC
	seedBucket := time.Unix(1660989600, 0).UTC()
	s.Require().NoError(s.db.DB.Exec(
		`INSERT INTO user_message_hourly (chat_id, user_id, user_name, bucket_ts, message_count, updated_at)
		 VALUES (?, 100, 'alice', ?, 5, CURRENT_TIMESTAMP)`,
		chatID, seedBucket,
	).Error)
	s.Require().NoError(s.db.DB.Exec(
		`INSERT INTO user_message_stats (chat_id, user_id, user_name, last_message_at, total_messages, updated_at)
		 VALUES (?, 100, 'alice', ?, 5, CURRENT_TIMESTAMP)`,
		chatID, seedBucket,
	).Error)

	// Import 2 messages from alice in the same UTC hour bucket (10:00 UTC).
	// epoch 1660990200 = 2022-08-20T10:10 UTC = 12:10 CEST → bucket 10:00 UTC.
	// epoch 1660991400 = 2022-08-20T10:30 UTC = 12:30 CEST → bucket 10:00 UTC.
	// Max = 10:30 UTC = 12:30 CEST. Cutoff = truncate(12:30 CEST) = 12:00 CEST = 10:00 UTC.
	// Both messages' bucket (10:00 UTC) ≤ cutoff (10:00 UTC) → included.
	// Also add a later-hour sentinel so max is pushed out and alice is definitely included.
	// epoch 1660993500 = 2022-08-20T11:05 UTC = 13:05 CEST → bucket 11:00 UTC.
	// Max = 11:05 UTC. Cutoff = truncate(13:05 CEST) = 13:00 CEST = 11:00 UTC.
	// Alice bucket 10:00 UTC ≤ 11:00 UTC → included.
	export := minimalExport(chatID, []exportMessage{
		{ID: 1, Type: "message", DateUnixtime: "1660990200", From: "alice", FromID: "user100"},
		{ID: 2, Type: "message", DateUnixtime: "1660991400", From: "alice", FromID: "user100"},
		// Sentinel in a later hour to ensure cutoff is at 11:00 UTC (not 10:00 UTC).
		{ID: 3, Type: "message", DateUnixtime: "1660993500", From: "sentinel", FromID: "user999"},
	})

	// Verify seed is in place before import.
	var seedCount int64
	s.Require().NoError(s.db.DB.Raw(`SELECT COUNT(*) FROM user_message_hourly WHERE chat_id = ? AND user_id = 100`, chatID).Scan(&seedCount).Error)
	s.Require().Equal(int64(1), seedCount, "seed row must exist before import")

	result, err := s.service.ImportFromExport(ctx, chatID, strings.NewReader(export))
	s.Require().NoError(err)
	s.Assert().Equal(3, result.MessagesProcessed)

	// After import: hourly rows for alice should be exactly 1 (the newly written bucket).
	var hourlyCount int64
	s.Require().NoError(s.db.DB.Raw(`SELECT COUNT(*) FROM user_message_hourly WHERE chat_id = ? AND user_id = 100`, chatID).Scan(&hourlyCount).Error)
	s.Assert().Equal(int64(1), hourlyCount, "only one bucket row for alice after import")

	// Total for alice must be 2 (not 5+2=7).
	var total int64
	s.Require().NoError(
		s.db.DB.Raw(`SELECT total_messages FROM user_message_stats WHERE chat_id = ? AND user_id = 100`, chatID).
			Scan(&total).Error,
	)
	s.Assert().Equal(int64(2), total)
}

// TestImportSkipsServiceMessages verifies service-type entries do not affect
// the cutoff or the imported counts.
func (s *ImporterDBSuite) TestImportSkipsServiceMessages() {
	ctx := context.Background()
	const chatID = int64(-1001549724404)

	// The service message has a much later unix time; it must NOT shift the cutoff.
	// Only the real message (epoch 1660990200 = 10:10 UTC = 12:10 CEST) is used for the cutoff.
	// Max from type==message = 10:10 UTC = 12:10 CEST.
	// Cutoff = truncate(12:10 CEST) = 12:00 CEST = 10:00 UTC.
	// Alice's bucket = 10:00 UTC ≤ 10:00 UTC → included.
	export := minimalExport(chatID, []exportMessage{
		{ID: 1, Type: "service", DateUnixtime: "1661018400", From: "", FromID: "channel1549724404"},
		// epoch 1660990200 = 2022-08-20T10:10:00 UTC = 12:10 CEST
		{ID: 2, Type: "message", DateUnixtime: "1660990200", From: "alice", FromID: "user100"},
	})

	result, err := s.service.ImportFromExport(ctx, chatID, strings.NewReader(export))
	s.Require().NoError(err)
	s.Assert().Equal(1, result.MessagesProcessed)

	var count int64
	s.Require().NoError(
		s.db.DB.Raw(`SELECT COUNT(*) FROM user_message_stats WHERE chat_id = ?`, chatID).Scan(&count).Error,
	)
	s.Assert().Equal(int64(1), count)
}

// TestImportChatIDFromExport verifies that when chatID=0 the export's own ID
// is used.
func (s *ImporterDBSuite) TestImportChatIDFromExport() {
	ctx := context.Background()
	const exportChatID = int64(1549724404)

	// epoch 1660990200 = 10:10 UTC = 12:10 CEST.
	// Max = 10:10 UTC → cutoff = truncate(12:10 CEST) = 12:00 CEST = 10:00 UTC.
	// Alice's bucket = 10:00 UTC ≤ 10:00 UTC → included.
	export := minimalExport(exportChatID, []exportMessage{
		{ID: 1, Type: "message", DateUnixtime: "1660990200", From: "alice", FromID: "user100"},
	})

	result, err := s.service.ImportFromExport(ctx, 0, strings.NewReader(export))
	s.Require().NoError(err)
	s.Assert().Equal(1, result.MessagesProcessed)

	var count int64
	s.Require().NoError(
		s.db.DB.Raw(`SELECT COUNT(*) FROM user_message_stats WHERE chat_id = ?`, exportChatID).Scan(&count).Error,
	)
	s.Assert().Equal(int64(1), count)
}
