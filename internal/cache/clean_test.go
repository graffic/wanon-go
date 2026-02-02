package cache

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/graffic/wanon-go/internal/testutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
)

func TestClean_DeletesOldCacheEntries(t *testing.T) {
	db := testutils.NewTestDB(t)

	// Create old cache entries
	oldTime := time.Now().Add(-72 * time.Hour).Unix()
	oldEntries := []CacheEntry{
		{ChatID: 1, MessageID: 1, Date: oldTime, Message: datatypes.JSON(`{"text":"old1"}`)},
		{ChatID: 1, MessageID: 2, Date: oldTime, Message: datatypes.JSON(`{"text":"old2"}`)},
	}
	for _, entry := range oldEntries {
		require.NoError(t, db.DB.Create(&entry).Error)
	}

	// Create recent cache entries
	recentTime := time.Now().Add(-1 * time.Hour).Unix()
	recentEntries := []CacheEntry{
		{ChatID: 1, MessageID: 3, Date: recentTime, Message: datatypes.JSON(`{"text":"recent1"}`)},
		{ChatID: 1, MessageID: 4, Date: recentTime, Message: datatypes.JSON(`{"text":"recent2"}`)},
	}
	for _, entry := range recentEntries {
		require.NoError(t, db.DB.Create(&entry).Error)
	}

	// Run clean with 48 hour retention
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	config := Config{
		CleanInterval: time.Hour,
		KeepDuration:  48 * time.Hour,
	}
	cleaner := NewCleaner(NewService(db.DB), config, logger)
	err := cleaner.CleanOnce(context.Background())

	require.NoError(t, err)

	// Verify old entries are deleted
	var count int64
	db.DB.Model(&CacheEntry{}).Where("date <= ?", oldTime).Count(&count)
	assert.Equal(t, int64(0), count)

	// Verify recent entries remain
	db.DB.Model(&CacheEntry{}).Count(&count)
	assert.Equal(t, int64(2), count)
}

func TestClean_NoEntriesToDelete(t *testing.T) {
	db := testutils.NewTestDB(t)

	// Create only recent entries
	recentTime := time.Now().Add(-1 * time.Hour).Unix()
	entry := CacheEntry{ChatID: 1, MessageID: 1, Date: recentTime, Message: datatypes.JSON(`{"text":"recent"}`)}
	require.NoError(t, db.DB.Create(&entry).Error)

	// Run clean with 48 hour retention
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	config := Config{
		CleanInterval: time.Hour,
		KeepDuration:  48 * time.Hour,
	}
	cleaner := NewCleaner(NewService(db.DB), config, logger)
	err := cleaner.CleanOnce(context.Background())

	require.NoError(t, err)

	// Verify entry still exists
	var count int64
	db.DB.Model(&CacheEntry{}).Count(&count)
	assert.Equal(t, int64(1), count)
}

func TestClean_EmptyCache(t *testing.T) {
	db := testutils.NewTestDB(t)

	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	config := Config{
		CleanInterval: time.Hour,
		KeepDuration:  48 * time.Hour,
	}
	cleaner := NewCleaner(NewService(db.DB), config, logger)
	err := cleaner.CleanOnce(context.Background())

	require.NoError(t, err)
}

func TestClean_CorrectRetentionCalculation(t *testing.T) {
	db := testutils.NewTestDB(t)

	// Create entry exactly at the threshold (48 hours ago)
	thresholdTime := time.Now().Add(-48 * time.Hour).Unix()
	entry := CacheEntry{ChatID: 1, MessageID: 1, Date: thresholdTime, Message: datatypes.JSON(`{"text":"threshold"}`)}
	require.NoError(t, db.DB.Create(&entry).Error)

	// Run clean with 48 hour retention
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	config := Config{
		CleanInterval: time.Hour,
		KeepDuration:  48 * time.Hour,
	}
	cleaner := NewCleaner(NewService(db.DB), config, logger)
	err := cleaner.CleanOnce(context.Background())

	require.NoError(t, err)
	// Entry at exactly 48 hours should be deleted
	var count int64
	db.DB.Model(&CacheEntry{}).Count(&count)
	assert.Equal(t, int64(0), count)
}

func TestCleaner_StartStop(t *testing.T) {
	db := testutils.NewTestDB(t)

	// Create old entries
	oldTime := time.Now().Add(-72 * time.Hour).Unix()
	entry := CacheEntry{ChatID: 1, MessageID: 1, Date: oldTime, Message: datatypes.JSON(`{"text":"old"}`)}
	require.NoError(t, db.DB.Create(&entry).Error)

	// Create cleaner with short interval for testing
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	config := Config{
		CleanInterval: 100 * time.Millisecond,
		KeepDuration:  48 * time.Hour,
	}
	cleaner := NewCleaner(NewService(db.DB), config, logger)

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
		assert.Equal(t, context.Canceled, err)
	case <-time.After(time.Second):
		t.Fatal("Cleaner did not stop in time")
	}

	// Verify old entries were cleaned
	var count int64
	db.DB.Model(&CacheEntry{}).Count(&count)
	assert.Equal(t, int64(0), count)
}
