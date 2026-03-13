package cache

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/graffic/wanon-go/internal/testutils"
	"gorm.io/datatypes"
)

type CleanIntegrationSuite struct {
	testutils.DBSuite
	service    *Service
	middleware *Middleware
	ctx        context.Context
}

func (s *CleanIntegrationSuite) SetupSuite() {
	s.DBSuite.SetupSuite()
	s.service = NewService(s.DB)
	s.middleware = NewMiddleware(s.service, slog.Default())
	s.ctx = context.Background()
}

func (s *CleanIntegrationSuite) TestClean_DeletesOldCacheEntries() {
	db := s.DB

	// Create old cache entries
	oldTime := time.Now().Add(-72 * time.Hour).Unix()
	oldEntries := []CacheEntry{
		{ChatID: 1, MessageID: 1, Date: oldTime, Message: datatypes.JSON(`{"text":"old1"}`)},
		{ChatID: 1, MessageID: 2, Date: oldTime, Message: datatypes.JSON(`{"text":"old2"}`)},
	}
	for _, entry := range oldEntries {
		s.Require().NoError(db.Create(&entry).Error)
	}

	// Create recent cache entries
	recentTime := time.Now().Add(-1 * time.Hour).Unix()
	recentEntries := []CacheEntry{
		{ChatID: 1, MessageID: 3, Date: recentTime, Message: datatypes.JSON(`{"text":"recent1"}`)},
		{ChatID: 1, MessageID: 4, Date: recentTime, Message: datatypes.JSON(`{"text":"recent2"}`)},
	}
	for _, entry := range recentEntries {
		s.Require().NoError(db.Create(&entry).Error)
	}

	// Run clean with 48 hour retention
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	config := Config{
		CleanInterval: time.Hour,
		KeepDuration:  48 * time.Hour,
	}
	cleaner := NewCleaner(NewService(db), config, logger)
	err := cleaner.CleanOnce(context.Background())

	s.Require().NoError(err)

	// Verify old entries are deleted
	var count int64
	db.Model(&CacheEntry{}).Where("date <= ?", oldTime).Count(&count)
	s.Assert().Equal(int64(0), count)

	// Verify recent entries remain
	db.Model(&CacheEntry{}).Count(&count)
	s.Assert().Equal(int64(2), count)
}

func (s *CacheIntegrationSuite) TestClean_NoEntriesToDelete() {
	db := s.DB

	// Create only recent entries
	recentTime := time.Now().Add(-1 * time.Hour).Unix()
	entry := CacheEntry{ChatID: 1, MessageID: 1, Date: recentTime, Message: datatypes.JSON(`{"text":"recent"}`)}
	s.Require().NoError(db.Create(&entry).Error)

	// Run clean with 48 hour retention
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	config := Config{
		CleanInterval: time.Hour,
		KeepDuration:  48 * time.Hour,
	}
	cleaner := NewCleaner(NewService(db), config, logger)
	err := cleaner.CleanOnce(context.Background())

	s.Require().NoError(err)

	// Verify entry still exists
	var count int64
	db.Model(&CacheEntry{}).Count(&count)
	s.Assert().Equal(int64(1), count)
}

func (s *CacheIntegrationSuite) TestClean_EmptyCache() {
	db := s.DB

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	config := Config{
		CleanInterval: time.Hour,
		KeepDuration:  48 * time.Hour,
	}
	cleaner := NewCleaner(NewService(db), config, logger)
	err := cleaner.CleanOnce(context.Background())

	s.Require().NoError(err)
}

func (s *CleanIntegrationSuite) TestClean_CorrectRetentionCalculation() {
	db := s.DB

	// Create entry just past the threshold (48 hours + 1 second ago)
	// Using strictly less than, so entry must be older than 48 hours to be deleted
	thresholdTime := time.Now().Add(-48*time.Hour - time.Second).Unix()
	entry := CacheEntry{ChatID: 1, MessageID: 1, Date: thresholdTime, Message: datatypes.JSON(`{"text":"threshold"}`)}
	s.Require().NoError(db.Create(&entry).Error)

	// Run clean with 48 hour retention
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	config := Config{
		CleanInterval: time.Hour,
		KeepDuration:  48 * time.Hour,
	}
	cleaner := NewCleaner(NewService(db), config, logger)
	err := cleaner.CleanOnce(context.Background())

	s.Require().NoError(err)
	// Entry older than 48 hours should be deleted
	var count int64
	db.Model(&CacheEntry{}).Count(&count)
	s.Assert().Equal(int64(0), count)
}

func (s *CleanIntegrationSuite) TestCleaner_StartStop() {
	db := s.DB

	// Create old entries
	oldTime := time.Now().Add(-72 * time.Hour).Unix()
	entry := CacheEntry{ChatID: 1, MessageID: 1, Date: oldTime, Message: datatypes.JSON(`{"text":"old"}`)}
	s.Require().NoError(db.Create(&entry).Error)

	// Create cleaner with short interval for testing
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	config := Config{
		CleanInterval: 100 * time.Millisecond,
		KeepDuration:  48 * time.Hour,
	}
	cleaner := NewCleaner(NewService(db), config, logger)

	// Start cleaner
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	done := make(chan error, 1)
	go func() {
		done <- cleaner.Start(ctx)
	}()

	// Wait for at least one clean cycle
	time.Sleep(200 * time.Millisecond)

	// Cancel context to stop cleaner
	cancel()

	// Wait for cleaner to stop
	select {
	case err := <-done:
		s.Assert().Equal(context.Canceled, err)
	case <-time.After(time.Second):
		s.T().Fatal("Cleaner did not stop in time")
	}

	// Verify old entries were cleaned
	var count int64
	db.Model(&CacheEntry{}).Count(&count)
	s.Assert().Equal(int64(0), count)
}
