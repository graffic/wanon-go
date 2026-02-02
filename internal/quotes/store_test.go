package quotes

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/graffic/wanon-go/internal/testutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
)

func TestStore_StoresQuoteWithEntries(t *testing.T) {
	db := testutils.NewTestDB(t)
	store := NewStore(db.DB)

	creator := map[string]interface{}{
		"id":         123,
		"first_name": "Test",
		"username":   "testuser",
	}
	creatorJSON, _ := json.Marshal(creator)

	entries := []CacheEntry{
		{Message: datatypes.JSON(`{"text":"first message"}`)},
		{Message: datatypes.JSON(`{"text":"second message"}`)},
		{Message: datatypes.JSON(`{"text":"third message"}`)},
	}

	quote, err := store.Store(context.Background(), StoreOptions{
		ChatID:  -100123,
		Creator: creator,
		Entries: entries,
	})
	require.NoError(t, err)

	// Verify quote was created
	assert.NotZero(t, quote.ID)
	assert.Equal(t, int64(-100123), quote.ChatID)
	assert.Equal(t, datatypes.JSON(creatorJSON), quote.Creator)
	assert.WithinDuration(t, time.Now(), quote.CreatedAt, time.Second)

	// Verify entries were stored
	var storedEntries []QuoteEntry
	err = db.DB.Where("quote_id = ?", quote.ID).Order("`order`").Find(&storedEntries).Error
	require.NoError(t, err)
	assert.Len(t, storedEntries, 3)

	// Verify order is correct (0, 1, 2)
	for i, entry := range storedEntries {
		assert.Equal(t, i, entry.Order)
	}
}

func TestStore_StoresSingleEntry(t *testing.T) {
	db := testutils.NewTestDB(t)
	store := NewStore(db.DB)

	creator := map[string]interface{}{"id": 123, "first_name": "Test"}

	entries := []CacheEntry{
		{Message: datatypes.JSON(`{"text":"single message"}`)},
	}

	quote, err := store.Store(context.Background(), StoreOptions{
		ChatID:  -100123,
		Creator: creator,
		Entries: entries,
	})
	require.NoError(t, err)

	var storedEntries []QuoteEntry
	err = db.DB.Where("quote_id = ?", quote.ID).Find(&storedEntries).Error
	require.NoError(t, err)
	assert.Len(t, storedEntries, 1)
	assert.Equal(t, 0, storedEntries[0].Order)
}

func TestStore_EmptyEntries(t *testing.T) {
	db := testutils.NewTestDB(t)
	store := NewStore(db.DB)

	creator := map[string]interface{}{"id": 123, "first_name": "Test"}

	_, err := store.Store(context.Background(), StoreOptions{
		ChatID:  -100123,
		Creator: creator,
		Entries: []CacheEntry{},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot store quote with no entries")
}

func TestStore_MultipleQuotesInSameChat(t *testing.T) {
	db := testutils.NewTestDB(t)
	store := NewStore(db.DB)

	creator := map[string]interface{}{"id": 123, "first_name": "Test"}

	// Store first quote
	entries1 := []CacheEntry{
		{Message: datatypes.JSON(`{"text":"quote 1"}`)},
	}
	quote1, err := store.Store(context.Background(), StoreOptions{
		ChatID:  -100123,
		Creator: creator,
		Entries: entries1,
	})
	require.NoError(t, err)

	// Store second quote
	entries2 := []CacheEntry{
		{Message: datatypes.JSON(`{"text":"quote 2"}`)},
	}
	quote2, err := store.Store(context.Background(), StoreOptions{
		ChatID:  -100123,
		Creator: creator,
		Entries: entries2,
	})
	require.NoError(t, err)

	// Verify both quotes exist
	var quotes []Quote
	err = db.DB.Where("chat_id = ?", -100123).Find(&quotes).Error
	require.NoError(t, err)
	assert.Len(t, quotes, 2)
	assert.NotEqual(t, quote1.ID, quote2.ID)
}

func TestStore_GetByID(t *testing.T) {
	db := testutils.NewTestDB(t)
	store := NewStore(db.DB)

	creator := map[string]interface{}{"id": 123, "first_name": "Test"}
	entries := []CacheEntry{
		{Message: datatypes.JSON(`{"text":"test message"}`)},
	}

	quote, err := store.Store(context.Background(), StoreOptions{
		ChatID:  -100123,
		Creator: creator,
		Entries: entries,
	})
	require.NoError(t, err)

	// Retrieve by ID
	retrieved, err := store.GetByID(context.Background(), quote.ID)
	require.NoError(t, err)
	assert.Equal(t, quote.ID, retrieved.ID)
	assert.Equal(t, quote.ChatID, retrieved.ChatID)
	assert.Len(t, retrieved.Entries, 1)
}

func TestStore_GetRandomForChat(t *testing.T) {
	db := testutils.NewTestDB(t)
	store := NewStore(db.DB)

	creator := map[string]interface{}{"id": 123, "first_name": "Test"}
	entries := []CacheEntry{
		{Message: datatypes.JSON(`{"text":"test message"}`)},
	}

	_, err := store.Store(context.Background(), StoreOptions{
		ChatID:  -100123,
		Creator: creator,
		Entries: entries,
	})
	require.NoError(t, err)

	// Get random quote
	retrieved, err := store.GetRandomForChat(context.Background(), -100123)
	require.NoError(t, err)
	assert.NotNil(t, retrieved)
	assert.Equal(t, int64(-100123), retrieved.ChatID)
}

func TestStore_GetRandomForChat_NoQuotes(t *testing.T) {
	db := testutils.NewTestDB(t)
	store := NewStore(db.DB)

	// Get random quote from empty chat
	retrieved, err := store.GetRandomForChat(context.Background(), -100123)
	require.NoError(t, err)
	assert.Nil(t, retrieved)
}

func TestStore_CountForChat(t *testing.T) {
	db := testutils.NewTestDB(t)
	store := NewStore(db.DB)

	creator := map[string]interface{}{"id": 123, "first_name": "Test"}
	entries := []CacheEntry{
		{Message: datatypes.JSON(`{"text":"test message"}`)},
	}

	// Initially empty
	count, err := store.CountForChat(context.Background(), -100123)
	require.NoError(t, err)
	assert.Equal(t, int64(0), count)

	// Add a quote
	_, err = store.Store(context.Background(), StoreOptions{
		ChatID:  -100123,
		Creator: creator,
		Entries: entries,
	})
	require.NoError(t, err)

	// Count should be 1
	count, err = store.CountForChat(context.Background(), -100123)
	require.NoError(t, err)
	assert.Equal(t, int64(1), count)
}

func TestStore_Delete(t *testing.T) {
	db := testutils.NewTestDB(t)
	store := NewStore(db.DB)

	creator := map[string]interface{}{"id": 123, "first_name": "Test"}
	entries := []CacheEntry{
		{Message: datatypes.JSON(`{"text":"test message"}`)},
	}

	quote, err := store.Store(context.Background(), StoreOptions{
		ChatID:  -100123,
		Creator: creator,
		Entries: entries,
	})
	require.NoError(t, err)

	// Delete the quote
	err = store.Delete(context.Background(), quote.ID)
	require.NoError(t, err)

	// Verify it's gone
	var count int64
	err = db.DB.Model(&Quote{}).Where("id = ?", quote.ID).Count(&count).Error
	require.NoError(t, err)
	assert.Equal(t, int64(0), count)
}

func TestStore_StoreFromBuild(t *testing.T) {
	db := testutils.NewTestDB(t)
	store := NewStore(db.DB)

	creator := map[string]interface{}{"id": 123, "first_name": "Test"}
	entries := []CacheEntry{
		{Message: datatypes.JSON(`{"text":"built message"}`)},
	}

	result := &BuildResult{
		ChatID:  -100123,
		Entries: entries,
	}

	quote, err := store.StoreFromBuild(context.Background(), creator, result)
	require.NoError(t, err)
	assert.NotNil(t, quote)
	assert.Equal(t, int64(-100123), quote.ChatID)
	assert.Len(t, quote.Entries, 1)
}
