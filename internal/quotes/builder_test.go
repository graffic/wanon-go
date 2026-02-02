package quotes

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/graffic/wanon-go/internal/testutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
)

func TestBuilder_BuildFrom_NoCacheEntries(t *testing.T) {
	db := testutils.NewTestDB(t)
	builder := NewBuilder(db.DB)

	// Try to build from a message that doesn't exist in cache
	result, err := builder.BuildFrom(context.Background(), -100123, 999)
	
	// Should return an error since no cache entries found
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "no cache entries found")
}

func TestBuilder_BuildFrom_OneCacheEntry(t *testing.T) {
	db := testutils.NewTestDB(t)

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
	require.NoError(t, db.DB.Create(&cacheEntry).Error)

	builder := NewBuilder(db.DB)
	result, err := builder.BuildFrom(context.Background(), -100123, 5)
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, result.Entries, 1)
	assert.Equal(t, int64(-100123), result.ChatID)

	var msgData MessageData
	err = json.Unmarshal(result.Entries[0].Message, &msgData)
	require.NoError(t, err)
	assert.Equal(t, "Cached message", msgData.Text)
}

func TestBuilder_BuildFrom_MultipleEntries(t *testing.T) {
	db := testutils.NewTestDB(t)

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
	require.NoError(t, db.DB.Create(&cacheEntry1).Error)

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
	require.NoError(t, db.DB.Create(&cacheEntry2).Error)

	msg3JSON, _ := json.Marshal(msg3)
	replyID3 := int64(2)
	cacheEntry3 := CacheEntry{
		ChatID:    -100123,
		MessageID: 3,
		ReplyID:   &replyID3,
		Date:      1609459100,
		Message:   datatypes.JSON(msg3JSON),
	}
	require.NoError(t, db.DB.Create(&cacheEntry3).Error)

	builder := NewBuilder(db.DB)
	result, err := builder.BuildFrom(context.Background(), -100123, 3)
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, result.Entries, 3)

	// Verify order (oldest first)
	var texts []string
	for _, entry := range result.Entries {
		var msgData MessageData
		err = json.Unmarshal(entry.Message, &msgData)
		require.NoError(t, err)
		texts = append(texts, msgData.Text)
	}
	assert.Equal(t, []string{"First", "Second", "Third"}, texts)
}

func TestBuilder_BuildFrom_PartialCache(t *testing.T) {
	db := testutils.NewTestDB(t)

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
	require.NoError(t, db.DB.Create(&cacheEntry2).Error)

	builder := NewBuilder(db.DB)
	result, err := builder.BuildFrom(context.Background(), -100123, 2)
	require.NoError(t, err)
	assert.NotNil(t, result)
	// Should only have the cached entry (msg1 not in cache)
	assert.Len(t, result.Entries, 1)

	var msgData MessageData
	err = json.Unmarshal(result.Entries[0].Message, &msgData)
	require.NoError(t, err)
	assert.Equal(t, "Second", msgData.Text)
}

func TestBuilder_BuildFrom_DifferentChat(t *testing.T) {
	db := testutils.NewTestDB(t)

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
	require.NoError(t, db.DB.Create(&cacheEntry).Error)

	builder := NewBuilder(db.DB)
	// Try to build from different chat
	result, err := builder.BuildFrom(context.Background(), -100123, 5)
	
	// Should return error since message not found in this chat
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "no cache entries found")
}

func TestBuilder_BuildFromMessage_UsesCache(t *testing.T) {
	db := testutils.NewTestDB(t)

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
	require.NoError(t, db.DB.Create(&cacheEntry).Error)

	builder := NewBuilder(db.DB)
	replyToID := int64(5)
	result, err := builder.BuildFromMessage(context.Background(), -100123, 10, &replyToID)
	require.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, result.Entries, 1)
}

func TestBuilder_BuildFromMessage_NotInCache(t *testing.T) {
	db := testutils.NewTestDB(t)

	builder := NewBuilder(db.DB)
	// Message not in cache, no reply to follow
	result, err := builder.BuildFromMessage(context.Background(), -100123, 10, nil)
	
	require.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "no cache entries found")
}

func TestExtractMessageData(t *testing.T) {
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
	require.NoError(t, err)
	assert.Equal(t, int64(1), data.MessageID)
	assert.Equal(t, "Hello", data.Text)
	assert.Equal(t, int64(1609459100), data.Date)
}
