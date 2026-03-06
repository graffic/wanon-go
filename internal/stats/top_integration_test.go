package stats_test

import (
	"context"
	"testing"
	"time"

	"github.com/graffic/wanon-go/internal/stats"
	"github.com/graffic/wanon-go/internal/testutils"
)

func TestGetTopUsersTotal(t *testing.T) {
	tdb := testutils.NewTestDB(t)
	svc := stats.NewService(tdb.DB)
	ctx := context.Background()

	chatID := int64(-1001)
	now := time.Now().UTC()

	// Record messages for several users
	for i := 0; i < 10; i++ {
		if err := svc.RecordMessage(ctx, chatID, 1, "alice", now); err != nil {
			t.Fatalf("RecordMessage: %v", err)
		}
	}
	for i := 0; i < 5; i++ {
		if err := svc.RecordMessage(ctx, chatID, 2, "bob", now); err != nil {
			t.Fatalf("RecordMessage: %v", err)
		}
	}
	for i := 0; i < 3; i++ {
		if err := svc.RecordMessage(ctx, chatID, 3, "charlie", now); err != nil {
			t.Fatalf("RecordMessage: %v", err)
		}
	}

	result, err := svc.GetTopUsersTotal(ctx, chatID, 2)
	if err != nil {
		t.Fatalf("GetTopUsersTotal: %v", err)
	}

	if len(result) != 2 {
		t.Fatalf("expected 2 users, got %d", len(result))
	}
	if result[0].UserName != "alice" || result[0].MessageCount != 10 {
		t.Errorf("expected alice with 10 messages, got %s with %d", result[0].UserName, result[0].MessageCount)
	}
	if result[1].UserName != "bob" || result[1].MessageCount != 5 {
		t.Errorf("expected bob with 5 messages, got %s with %d", result[1].UserName, result[1].MessageCount)
	}
}

func TestGetTopUsersSince(t *testing.T) {
	tdb := testutils.NewTestDB(t)
	svc := stats.NewService(tdb.DB)
	ctx := context.Background()

	chatID := int64(-1002)
	now := time.Now().UTC()
	yesterday := now.Add(-24 * time.Hour)

	// Record messages: alice yesterday, bob today
	for i := 0; i < 5; i++ {
		if err := svc.RecordMessage(ctx, chatID, 1, "alice", yesterday); err != nil {
			t.Fatalf("RecordMessage: %v", err)
		}
	}
	for i := 0; i < 3; i++ {
		if err := svc.RecordMessage(ctx, chatID, 2, "bob", now); err != nil {
			t.Fatalf("RecordMessage: %v", err)
		}
	}

	// Query since today start - should only include bob
	todayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	result, err := svc.GetTopUsersSince(ctx, chatID, todayStart, 10)
	if err != nil {
		t.Fatalf("GetTopUsersSince: %v", err)
	}

	if len(result) != 1 {
		t.Fatalf("expected 1 user, got %d", len(result))
	}
	if result[0].UserName != "bob" || result[0].MessageCount != 3 {
		t.Errorf("expected bob with 3 messages, got %s with %d", result[0].UserName, result[0].MessageCount)
	}

	// Query since yesterday - should include both
	result, err = svc.GetTopUsersSince(ctx, chatID, yesterday.Truncate(time.Hour), 10)
	if err != nil {
		t.Fatalf("GetTopUsersSince: %v", err)
	}

	if len(result) != 2 {
		t.Fatalf("expected 2 users, got %d", len(result))
	}
	if result[0].UserName != "alice" || result[0].MessageCount != 5 {
		t.Errorf("expected alice with 5 messages first, got %s with %d", result[0].UserName, result[0].MessageCount)
	}
}

func TestGetTopUsersSinceMergesNameChanges(t *testing.T) {
	tdb := testutils.NewTestDB(t)
	svc := stats.NewService(tdb.DB)
	ctx := context.Background()

	chatID := int64(-1010)
	now := time.Now().UTC()
	// Place messages in two different hourly buckets so they create separate
	// hourly rows with different user_name values for the same user_id.
	bucket1 := now.Add(-2 * time.Hour)
	bucket2 := now

	// Same user_id (42) but different names in each bucket.
	for i := 0; i < 3; i++ {
		if err := svc.RecordMessage(ctx, chatID, 42, "Alice Wonderland", bucket1); err != nil {
			t.Fatalf("RecordMessage: %v", err)
		}
	}
	for i := 0; i < 4; i++ {
		if err := svc.RecordMessage(ctx, chatID, 42, "alicew", bucket2); err != nil {
			t.Fatalf("RecordMessage: %v", err)
		}
	}

	since := bucket1.Truncate(time.Hour)
	result, err := svc.GetTopUsersSince(ctx, chatID, since, 10)
	if err != nil {
		t.Fatalf("GetTopUsersSince: %v", err)
	}

	// The user should appear as a single entry with combined count.
	if len(result) != 1 {
		t.Fatalf("expected 1 user (merged), got %d: %+v", len(result), result)
	}
	if result[0].MessageCount != 7 {
		t.Errorf("expected 7 messages, got %d", result[0].MessageCount)
	}
	// Should use the most recent name.
	if result[0].UserName != "alicew" {
		t.Errorf("expected most recent name 'alicew', got %q", result[0].UserName)
	}
}

func TestGetTopUsersSinceEmpty(t *testing.T) {
	tdb := testutils.NewTestDB(t)
	svc := stats.NewService(tdb.DB)
	ctx := context.Background()

	result, err := svc.GetTopUsersSince(ctx, -9999, time.Now().UTC(), 5)
	if err != nil {
		t.Fatalf("GetTopUsersSince: %v", err)
	}
	if len(result) != 0 {
		t.Fatalf("expected 0 users, got %d", len(result))
	}
}

func TestGetTopUsersTotalIsolatedByChat(t *testing.T) {
	tdb := testutils.NewTestDB(t)
	svc := stats.NewService(tdb.DB)
	ctx := context.Background()

	now := time.Now().UTC()

	// Record messages in two different chats
	for i := 0; i < 5; i++ {
		if err := svc.RecordMessage(ctx, -1003, 1, "alice", now); err != nil {
			t.Fatalf("RecordMessage: %v", err)
		}
	}
	for i := 0; i < 3; i++ {
		if err := svc.RecordMessage(ctx, -1004, 2, "bob", now); err != nil {
			t.Fatalf("RecordMessage: %v", err)
		}
	}

	// Query chat -1003 should only have alice
	result, err := svc.GetTopUsersTotal(ctx, -1003, 10)
	if err != nil {
		t.Fatalf("GetTopUsersTotal: %v", err)
	}
	if len(result) != 1 {
		t.Fatalf("expected 1 user in chat -1003, got %d", len(result))
	}
	if result[0].UserName != "alice" {
		t.Errorf("expected alice, got %s", result[0].UserName)
	}
}
