package cache

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"time"

	"github.com/graffic/wanon-go/internal/testutils"
	"gorm.io/datatypes"
)

// Integration test with mock Telegram server
func (s *CacheDBSuite) TestCacheIntegration_FullFlow() {
	// Load fixtures
	fixture1 := testutils.LoadFixture(s.T(), "fixture.1.json")
	fixture2 := testutils.LoadFixture(s.T(), "fixture.2.edit.json")

	var updates1, updates2 []Update
	s.Require().NoError(json.Unmarshal(fixture1, &updates1))
	s.Require().NoError(json.Unmarshal(fixture2, &updates2))

	// Create mock Telegram API server
	requestCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount++

		var response map[string]interface{}
		switch r.URL.Path {
		case "/bottest-token/getUpdates":
			if requestCount == 1 {
				response = map[string]interface{}{
					"ok":     true,
					"result": updates1,
				}
			} else {
				response = map[string]interface{}{
					"ok":     true,
					"result": updates2,
				}
			}
		default:
			response = map[string]interface{}{"ok": true}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// Setup test database
	db := s.db

	// Create cache handlers
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	service := NewService(db.DB)
	adder := NewAddCommand(service, logger)
	editor := NewEditCommand(service, logger)

	// Process first batch of updates (add messages)
	for _, update := range updates1 {
		if update.Message != nil {
			msgJSON, _ := json.Marshal(update.Message)
			err := adder.Execute(context.Background(), msgJSON)
			s.Require().NoError(err)
		}
	}

	// Verify messages were added
	var count int64
	db.DB.Model(&CacheEntry{}).Count(&count)
	s.Assert().Equal(int64(2), count)

	// Process second batch (edit message)
	for _, update := range updates2 {
		if update.EditedMessage != nil {
			msgJSON, _ := json.Marshal(update.EditedMessage)
			err := editor.Execute(context.Background(), msgJSON)
			s.Require().NoError(err)
		}
	}

	// Verify message was edited
	var entry CacheEntry
	err := db.DB.First(&entry, "chat_id = ? AND message_id = ?", -1001234567890, 1).Error
	s.Require().NoError(err)

	var msg Message
	err = json.Unmarshal(entry.Message, &msg)
	s.Require().NoError(err)
	s.Assert().Equal("Hello, this is an edited test message", msg.Text)
}

func (s *CacheDBSuite) TestCacheIntegration_CleanOldEntries() {
	db := s.db

	// Create entries with different ages
	now := time.Now()

	oldEntries := []CacheEntry{
		{
			ChatID:    -100123,
			MessageID: 1,
			Date:      now.Add(-72 * time.Hour).Unix(),
			Message:   datatypes.JSON(`{"text":"old1"}`),
		},
		{
			ChatID:    -100123,
			MessageID: 2,
			Date:      now.Add(-49 * time.Hour).Unix(),
			Message:   datatypes.JSON(`{"text":"old2"}`),
		},
	}

	recentEntries := []CacheEntry{
		{
			ChatID:    -100123,
			MessageID: 3,
			Date:      now.Add(-1 * time.Hour).Unix(),
			Message:   datatypes.JSON(`{"text":"recent1"}`),
		},
		{
			ChatID:    -100123,
			MessageID: 4,
			Date:      now.Add(-23 * time.Hour).Unix(),
			Message:   datatypes.JSON(`{"text":"recent2"}`),
		},
	}

	for _, entry := range oldEntries {
		s.Require().NoError(db.DB.Create(&entry).Error)
	}
	for _, entry := range recentEntries {
		s.Require().NoError(db.DB.Create(&entry).Error)
	}

	// Run cleaner with 48 hour retention
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	config := Config{
		CleanInterval: time.Hour,
		KeepDuration:  48 * time.Hour,
	}
	cleaner := NewCleaner(NewService(db.DB), config, logger)
	err := cleaner.CleanOnce(context.Background())

	s.Require().NoError(err)

	// Verify only recent entries remain
	var count int64
	db.DB.Model(&CacheEntry{}).Count(&count)
	s.Assert().Equal(int64(2), count)

	// Verify specific entries
	var entries []CacheEntry
	err = db.DB.Find(&entries).Error
	s.Require().NoError(err)

	ids := make([]int64, len(entries))
	for i, e := range entries {
		ids[i] = e.MessageID
	}
	s.Assert().Contains(ids, int64(3))
	s.Assert().Contains(ids, int64(4))
}

// Update represents a Telegram update (for integration tests)
type Update struct {
	UpdateID      int64           `json:"update_id"`
	Message       json.RawMessage `json:"message,omitempty"`
	EditedMessage json.RawMessage `json:"edited_message,omitempty"`
}
