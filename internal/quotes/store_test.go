package quotes

import (
	"context"
	"encoding/json"
	"time"

	"testing"

	"github.com/graffic/wanon-go/internal/testutils"
	"github.com/stretchr/testify/suite"
	"gorm.io/datatypes"
)

type StoreDBSuite struct {
	testutils.DBSuite
}

func (s *StoreDBSuite) SetupSuite() {
	s.DBSuite.SetupSuite()
}

func TestStoreDBSuite(t *testing.T) {
	suite.Run(t, new(StoreDBSuite))
}

func (s *StoreDBSuite) TestStore_StoresQuoteWithEntries() {
	store := NewStore(s.DB)

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
	s.Require().NoError(err)

	// Verify quote was created
	s.Assert().NotZero(quote.ID)
	s.Assert().Equal(int64(-100123), quote.ChatID)
	// Compare JSON by unmarshaling both to compare the data
	var expectedCreator, actualCreator map[string]interface{}
	json.Unmarshal(creatorJSON, &expectedCreator)
	json.Unmarshal(quote.Creator, &actualCreator)
	s.Assert().Equal(expectedCreator, actualCreator)
	s.Assert().WithinDuration(time.Now(), quote.CreatedAt, time.Second)

	// Verify entries were stored
	var storedEntries []QuoteEntry
	err = s.DB.Where("quote_id = ?", quote.ID).Order("\"order\"").Find(&storedEntries).Error
	s.Require().NoError(err)
	s.Assert().Len(storedEntries, 3)

	// Verify order is correct (0, 1, 2)
	for i, entry := range storedEntries {
		s.Assert().Equal(i, entry.Order)
	}
}

func (s *StoreDBSuite) TestStore_StoresSingleEntry() {
	store := NewStore(s.DB)

	creator := map[string]interface{}{"id": 123, "first_name": "Test"}

	entries := []CacheEntry{
		{Message: datatypes.JSON(`{"text":"single message"}`)},
	}

	quote, err := store.Store(context.Background(), StoreOptions{
		ChatID:  -100123,
		Creator: creator,
		Entries: entries,
	})
	s.Require().NoError(err)

	var storedEntries []QuoteEntry
	err = s.DB.Where("quote_id = ?", quote.ID).Find(&storedEntries).Error
	s.Require().NoError(err)
	s.Assert().Len(storedEntries, 1)
	s.Assert().Equal(0, storedEntries[0].Order)
}

func (s *StoreDBSuite) TestStore_EmptyEntries() {
	store := NewStore(s.DB)

	creator := map[string]interface{}{"id": 123, "first_name": "Test"}

	_, err := store.Store(context.Background(), StoreOptions{
		ChatID:  -100123,
		Creator: creator,
		Entries: []CacheEntry{},
	})
	s.Require().Error(err)
	s.Assert().Contains(err.Error(), "cannot store quote with no entries")
}

func (s *StoreDBSuite) TestStore_MultipleQuotesInSameChat() {
	store := NewStore(s.DB)

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
	s.Require().NoError(err)

	// Store second quote
	entries2 := []CacheEntry{
		{Message: datatypes.JSON(`{"text":"quote 2"}`)},
	}
	quote2, err := store.Store(context.Background(), StoreOptions{
		ChatID:  -100123,
		Creator: creator,
		Entries: entries2,
	})
	s.Require().NoError(err)

	// Verify both quotes exist
	var quotes []Quote
	err = s.DB.Where("chat_id = ?", -100123).Find(&quotes).Error
	s.Require().NoError(err)
	s.Assert().Len(quotes, 2)
	s.Assert().NotEqual(quote1.ID, quote2.ID)
}

func (s *StoreDBSuite) TestStore_GetByID() {
	store := NewStore(s.DB)

	creator := map[string]interface{}{"id": 123, "first_name": "Test"}
	entries := []CacheEntry{
		{Message: datatypes.JSON(`{"text":"test message"}`)},
	}

	quote, err := store.Store(context.Background(), StoreOptions{
		ChatID:  -100123,
		Creator: creator,
		Entries: entries,
	})
	s.Require().NoError(err)

	// Retrieve by ID
	retrieved, err := store.GetByID(context.Background(), quote.ID)
	s.Require().NoError(err)
	s.Assert().Equal(quote.ID, retrieved.ID)
	s.Assert().Equal(quote.ChatID, retrieved.ChatID)
	s.Assert().Len(retrieved.Entries, 1)
}

func (s *StoreDBSuite) TestStore_GetRandomForChat() {
	store := NewStore(s.DB)

	creator := map[string]interface{}{"id": 123, "first_name": "Test"}
	entries := []CacheEntry{
		{Message: datatypes.JSON(`{"text":"test message"}`)},
	}

	_, err := store.Store(context.Background(), StoreOptions{
		ChatID:  -100123,
		Creator: creator,
		Entries: entries,
	})
	s.Require().NoError(err)

	// Get random quote
	retrieved, err := store.GetRandomForChat(context.Background(), -100123)
	s.Require().NoError(err)
	s.Assert().NotNil(retrieved)
	s.Assert().Equal(int64(-100123), retrieved.ChatID)
}

func (s *StoreDBSuite) TestStore_GetRandomForChat_NoQuotes() {
	store := NewStore(s.DB)

	// Get random quote from empty chat
	retrieved, err := store.GetRandomForChat(context.Background(), -100123)
	s.Require().NoError(err)
	s.Assert().Nil(retrieved)
}

func (s *StoreDBSuite) TestStore_CountForChat() {
	store := NewStore(s.DB)

	creator := map[string]interface{}{"id": 123, "first_name": "Test"}
	entries := []CacheEntry{
		{Message: datatypes.JSON(`{"text":"test message"}`)},
	}

	// Initially empty
	count, err := store.CountForChat(context.Background(), -100123)
	s.Require().NoError(err)
	s.Assert().Equal(int64(0), count)

	// Add a quote
	_, err = store.Store(context.Background(), StoreOptions{
		ChatID:  -100123,
		Creator: creator,
		Entries: entries,
	})
	s.Require().NoError(err)

	// Count should be 1
	count, err = store.CountForChat(context.Background(), -100123)
	s.Require().NoError(err)
	s.Assert().Equal(int64(1), count)
}

func (s *StoreDBSuite) TestStore_Delete() {
	store := NewStore(s.DB)

	creator := map[string]interface{}{"id": 123, "first_name": "Test"}
	entries := []CacheEntry{
		{Message: datatypes.JSON(`{"text":"test message"}`)},
	}

	quote, err := store.Store(context.Background(), StoreOptions{
		ChatID:  -100123,
		Creator: creator,
		Entries: entries,
	})
	s.Require().NoError(err)

	// Delete the quote
	err = store.Delete(context.Background(), quote.ID)
	s.Require().NoError(err)

	// Verify it's gone
	var count int64
	err = s.DB.Model(&Quote{}).Where("id = ?", quote.ID).Count(&count).Error
	s.Require().NoError(err)
	s.Assert().Equal(int64(0), count)
}

func (s *StoreDBSuite) TestStore_StoreFromBuild() {
	store := NewStore(s.DB)

	creator := map[string]interface{}{"id": 123, "first_name": "Test"}
	entries := []CacheEntry{
		{Message: datatypes.JSON(`{"text":"built message"}`)},
	}

	result := &BuildResult{
		ChatID:  -100123,
		Entries: entries,
	}

	quote, err := store.StoreFromBuild(context.Background(), creator, result)
	s.Require().NoError(err)
	s.Assert().NotNil(quote)
	s.Assert().Equal(int64(-100123), quote.ChatID)
	s.Assert().Len(quote.Entries, 1)
}
