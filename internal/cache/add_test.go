package cache

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"testing"

	"github.com/graffic/wanon-go/internal/testutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAdd_StoresMessageInCache(t *testing.T) {
	db := testutils.NewTestDB(t)
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	adder := NewAddCommand(NewService(db.DB), logger)

	message := Message{
		MessageID: 1,
		Chat:      Chat{ID: 123},
		From:      &User{ID: 456, FirstName: "Test"},
		Date:      1609459200,
		Text:      "Test message",
	}
	messageJSON, _ := json.Marshal(message)

	err := adder.Execute(context.Background(), messageJSON)
	require.NoError(t, err)

	// Verify entry was created
	var entry CacheEntry
	err = db.DB.First(&entry, "chat_id = ? AND message_id = ?", 123, 1).Error
	require.NoError(t, err)

	assert.Equal(t, int64(123), entry.ChatID)
	assert.Equal(t, int64(1), entry.MessageID)
	assert.Equal(t, int64(1609459200), entry.Date)
	assert.NotNil(t, entry.Message)
}

func TestAdd_StoresReplyID(t *testing.T) {
	db := testutils.NewTestDB(t)
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	adder := NewAddCommand(NewService(db.DB), logger)

	replyID := int64(5)
	message := Message{
		MessageID: 1,
		Chat:      Chat{ID: 123},
		From:      &User{ID: 456, FirstName: "Test"},
		Date:      1609459200,
		Text:      "Reply message",
		ReplyTo: &Message{
			MessageID: 5,
		},
	}
	messageJSON, _ := json.Marshal(message)

	err := adder.Execute(context.Background(), messageJSON)
	require.NoError(t, err)

	var entry CacheEntry
	err = db.DB.First(&entry, "chat_id = ? AND message_id = ?", 123, 1).Error
	require.NoError(t, err)

	assert.NotNil(t, entry.ReplyID)
	assert.Equal(t, replyID, *entry.ReplyID)
}

func TestAdd_StoresFullMessageJSON(t *testing.T) {
	db := testutils.NewTestDB(t)
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	adder := NewAddCommand(NewService(db.DB), logger)

	message := Message{
		MessageID: 1,
		Chat:      Chat{ID: 123},
		From:      &User{ID: 456, FirstName: "Test", Username: "testuser"},
		Date:      1609459200,
		Text:      "Test message",
	}
	messageJSON, _ := json.Marshal(message)

	err := adder.Execute(context.Background(), messageJSON)
	require.NoError(t, err)

	var entry CacheEntry
	err = db.DB.First(&entry, "chat_id = ? AND message_id = ?", 123, 1).Error
	require.NoError(t, err)

	// Verify message JSON
	var storedMessage Message
	err = json.Unmarshal(entry.Message, &storedMessage)
	require.NoError(t, err)

	assert.Equal(t, message.MessageID, storedMessage.MessageID)
	assert.Equal(t, message.Text, storedMessage.Text)
}

func TestAdd_DuplicateMessageUpdates(t *testing.T) {
	db := testutils.NewTestDB(t)
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	adder := NewAddCommand(NewService(db.DB), logger)

	// First add
	message1 := Message{
		MessageID: 1,
		Chat:      Chat{ID: 123},
		From:      &User{ID: 456, FirstName: "Test"},
		Date:      1609459200,
		Text:      "Original",
	}
	message1JSON, _ := json.Marshal(message1)
	err := adder.Execute(context.Background(), message1JSON)
	require.NoError(t, err)

	// Second add with same IDs but different content
	message2 := Message{
		MessageID: 1,
		Chat:      Chat{ID: 123},
		From:      &User{ID: 456, FirstName: "Test"},
		Date:      1609459200,
		Text:      "Updated",
	}
	message2JSON, _ := json.Marshal(message2)
	err = adder.Execute(context.Background(), message2JSON)
	require.NoError(t, err)

	// Verify only one entry exists with updated content
	var entries []CacheEntry
	err = db.DB.Where("chat_id = ? AND message_id = ?", 123, 1).Find(&entries).Error
	require.NoError(t, err)
	assert.Len(t, entries, 1)

	var storedMessage Message
	err = json.Unmarshal(entries[0].Message, &storedMessage)
	require.NoError(t, err)
	assert.Equal(t, "Updated", storedMessage.Text)
}
