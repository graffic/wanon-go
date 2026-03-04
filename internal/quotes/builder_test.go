package quotes

import (
	"context"
	"encoding/json"

	"gorm.io/datatypes"
)

func (s *QuotesDBSuite) TestBuilder_BuildFrom_NoCacheEntries() {
	builder := NewBuilder(s.db.DB)

	// Try to build from a message that doesn't exist in cache
	result, err := builder.BuildFrom(context.Background(), -100123, 999)

	// Should return an error since no cache entries found
	s.Require().Error(err)
	s.Assert().Nil(result)
	s.Assert().Contains(err.Error(), "no cache entries found")
}

func (s *QuotesDBSuite) TestBuilder_BuildFrom_OneCacheEntry() {
	// Add a message to cache
	cachedMsg := map[string]interface{}{
		"message_id": float64(5),
		"chat":       map[string]interface{}{"id": float64(-100123)},
		"date":       float64(1609459100),
		"text":       "Cached message",
		"from":       map[string]interface{}{"id": float64(456), "first_name": "Cached"},
	}
	msgJSON, _ := json.Marshal(cachedMsg)
	cacheEntry := CacheEntry{
		ChatID:    -100123,
		MessageID: 5,
		Date:      1609459100,
		Message:   datatypes.JSON(msgJSON),
	}
	s.Require().NoError(s.db.DB.Create(&cacheEntry).Error)

	builder := NewBuilder(s.db.DB)
	result, err := builder.BuildFrom(context.Background(), -100123, 5)
	s.Require().NoError(err)
	s.Assert().NotNil(result)
	s.Assert().Len(result.Entries, 1)
	s.Assert().Equal(int64(-100123), result.ChatID)

	var msgData MessageData
	err = json.Unmarshal(result.Entries[0].Message, &msgData)
	s.Require().NoError(err)
	s.Assert().Equal("Cached message", msgData.Text)
}

func (s *QuotesDBSuite) TestBuilder_BuildFrom_MultipleEntries() {
	// Create a chain: msg1 -> msg2 -> msg3
	msg1 := map[string]interface{}{
		"message_id": float64(1),
		"chat":       map[string]interface{}{"id": float64(-100123)},
		"date":       float64(1609459000),
		"text":       "First",
		"from":       map[string]interface{}{"id": float64(1)},
	}
	msg2 := map[string]interface{}{
		"message_id": float64(2),
		"chat":       map[string]interface{}{"id": float64(-100123)},
		"date":       float64(1609459050),
		"text":       "Second",
		"from":       map[string]interface{}{"id": float64(2)},
	}
	msg3 := map[string]interface{}{
		"message_id": float64(3),
		"chat":       map[string]interface{}{"id": float64(-100123)},
		"date":       float64(1609459100),
		"text":       "Third",
		"from":       map[string]interface{}{"id": float64(3)},
	}

	// msg2 replies to msg1
	msg1JSON, _ := json.Marshal(msg1)
	cacheEntry1 := CacheEntry{
		ChatID:    -100123,
		MessageID: 1,
		Date:      1609459000,
		Message:   datatypes.JSON(msg1JSON),
	}
	s.Require().NoError(s.db.DB.Create(&cacheEntry1).Error)

	// msg3 replies to msg2
	msg2JSON, _ := json.Marshal(msg2)
	replyID2 := int64(1)
	cacheEntry2 := CacheEntry{
		ChatID:    -100123,
		MessageID: 2,
		ReplyID:   &replyID2,
		Date:      1609459050,
		Message:   datatypes.JSON(msg2JSON),
	}
	s.Require().NoError(s.db.DB.Create(&cacheEntry2).Error)

	msg3JSON, _ := json.Marshal(msg3)
	replyID3 := int64(2)
	cacheEntry3 := CacheEntry{
		ChatID:    -100123,
		MessageID: 3,
		ReplyID:   &replyID3,
		Date:      1609459100,
		Message:   datatypes.JSON(msg3JSON),
	}
	s.Require().NoError(s.db.DB.Create(&cacheEntry3).Error)

	builder := NewBuilder(s.db.DB)
	result, err := builder.BuildFrom(context.Background(), -100123, 3)
	s.Require().NoError(err)
	s.Assert().NotNil(result)
	s.Assert().Len(result.Entries, 3)

	// Verify order (oldest first)
	var texts []string
	for _, entry := range result.Entries {
		var msgData MessageData
		err = json.Unmarshal(entry.Message, &msgData)
		s.Require().NoError(err)
		texts = append(texts, msgData.Text)
	}
	s.Assert().Equal([]string{"First", "Second", "Third"}, texts)
}

func (s *QuotesDBSuite) TestBuilder_BuildFrom_PartialCache() {
	// Only cache msg2, not msg1
	msg2 := map[string]interface{}{
		"message_id": float64(2),
		"chat":       map[string]interface{}{"id": float64(-100123)},
		"date":       float64(1609459050),
		"text":       "Second",
		"from":       map[string]interface{}{"id": float64(2)},
	}
	msg2JSON, _ := json.Marshal(msg2)
	replyID2 := int64(1)
	cacheEntry2 := CacheEntry{
		ChatID:    -100123,
		MessageID: 2,
		ReplyID:   &replyID2,
		Date:      1609459050,
		Message:   datatypes.JSON(msg2JSON),
	}
	s.Require().NoError(s.db.DB.Create(&cacheEntry2).Error)

	builder := NewBuilder(s.db.DB)
	result, err := builder.BuildFrom(context.Background(), -100123, 2)
	s.Require().NoError(err)
	s.Assert().NotNil(result)
	// Should only have the cached entry (msg1 not in cache)
	s.Assert().Len(result.Entries, 1)

	var msgData MessageData
	err = json.Unmarshal(result.Entries[0].Message, &msgData)
	s.Require().NoError(err)
	s.Assert().Equal("Second", msgData.Text)
}

func (s *QuotesDBSuite) TestBuilder_BuildFrom_DifferentChat() {
	// Cache entry in different chat
	cachedMsg := map[string]interface{}{
		"message_id": float64(5),
		"chat":       map[string]interface{}{"id": float64(-100999)}, // Different chat
		"date":       float64(1609459100),
		"text":       "Cached in other chat",
	}
	msgJSON, _ := json.Marshal(cachedMsg)
	cacheEntry := CacheEntry{
		ChatID:    -100999,
		MessageID: 5,
		Date:      1609459100,
		Message:   datatypes.JSON(msgJSON),
	}
	s.Require().NoError(s.db.DB.Create(&cacheEntry).Error)

	builder := NewBuilder(s.db.DB)
	// Try to build from different chat
	result, err := builder.BuildFrom(context.Background(), -100123, 5)

	// Should return error since message not found in this chat
	s.Require().Error(err)
	s.Assert().Nil(result)
	s.Assert().Contains(err.Error(), "no cache entries found")
}

func (s *QuotesDBSuite) TestBuilder_BuildFromMessage_UsesCache() {
	// Add a message to cache
	cachedMsg := map[string]interface{}{
		"message_id": float64(5),
		"chat":       map[string]interface{}{"id": float64(-100123)},
		"date":       float64(1609459100),
		"text":       "Cached message",
		"from":       map[string]interface{}{"id": float64(456), "first_name": "Cached"},
	}
	msgJSON, _ := json.Marshal(cachedMsg)
	cacheEntry := CacheEntry{
		ChatID:    -100123,
		MessageID: 5,
		Date:      1609459100,
		Message:   datatypes.JSON(msgJSON),
	}
	s.Require().NoError(s.db.DB.Create(&cacheEntry).Error)

	builder := NewBuilder(s.db.DB)
	replyToID := int64(5)
	result, err := builder.BuildFromMessage(context.Background(), -100123, 10, &replyToID)
	s.Require().NoError(err)
	s.Assert().NotNil(result)
	s.Assert().Len(result.Entries, 1)
}

func (s *QuotesDBSuite) TestBuilder_BuildFromMessage_NotInCache() {
	builder := NewBuilder(s.db.DB)
	// Message not in cache, no reply to follow
	result, err := builder.BuildFromMessage(context.Background(), -100123, 10, nil)

	s.Require().Error(err)
	s.Assert().Nil(result)
	s.Assert().Contains(err.Error(), "no cache entries found")
}

func (s *QuotesDBSuite) TestExtractMessageData() {
	msg := map[string]interface{}{
		"message_id": float64(1),
		"from":       map[string]interface{}{"first_name": "Test"},
		"date":       float64(1609459100),
		"text":       "Hello",
		"chat":       map[string]interface{}{"id": float64(-100123)},
	}
	msgJSON, _ := json.Marshal(msg)
	entry := CacheEntry{
		Message: datatypes.JSON(msgJSON),
	}

	data, err := ExtractMessageData(entry)
	s.Require().NoError(err)
	s.Assert().Equal(int64(1), data.MessageID)
	s.Assert().Equal("Hello", data.Text)
	s.Assert().Equal(int64(1609459100), data.Date)
}
