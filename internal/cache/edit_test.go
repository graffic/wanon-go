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
	"gorm.io/datatypes"
)

func TestEdit_UpdatesExistingMessage(t *testing.T) {
	db := testutils.NewTestDB(t)

	// First add a message
	originalMessage := Message{
		MessageID: 1,
		Chat:      Chat{ID: 123},
		From:      &User{ID: 456, FirstName: "Test"},
		Date:      1609459200,
		Text:      "Original text",
	}
	originalJSON, _ := json.Marshal(originalMessage)
	entry := CacheEntry{
		ChatID:    123,
		MessageID: 1,
		Date:      1609459200,
		Message:   datatypes.JSON(originalJSON),
	}
	require.NoError(t, db.DB.Create(&entry).Error)

	// Now edit the message
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	editor := NewEditCommand(NewService(db.DB), logger)
	editedMessage := EditedMessage{
		MessageID: 1,
		Chat:      Chat{ID: 123},
		Date:      1609459200,
		EditDate:  1609459260,
		Text:      "Edited text",
	}
	editedJSON, _ := json.Marshal(editedMessage)

	err := editor.Execute(context.Background(), editedJSON)
	require.NoError(t, err)

	// Verify the message was updated
	var updatedEntry CacheEntry
	err = db.DB.First(&updatedEntry, "chat_id = ? AND message_id = ?", 123, 1).Error
	require.NoError(t, err)

	var storedMessage Message
	err = json.Unmarshal(updatedEntry.Message, &storedMessage)
	require.NoError(t, err)
	assert.Equal(t, "Edited text", storedMessage.Text)
}

func TestEdit_NonExistentMessage(t *testing.T) {
	db := testutils.NewTestDB(t)
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	editor := NewEditCommand(NewService(db.DB), logger)

	// Try to edit a message that doesn't exist
	editedMessage := EditedMessage{
		MessageID: 999,
		Chat:      Chat{ID: 123},
		Date:      1609459200,
		EditDate:  1609459260,
		Text:      "Edited text",
	}
	editedJSON, _ := json.Marshal(editedMessage)

	err := editor.Execute(context.Background(), editedJSON)
	// Should not error, just no-op
	require.NoError(t, err)

	// Verify no entries exist
	var count int64
	db.DB.Model(&CacheEntry{}).Count(&count)
	assert.Equal(t, int64(0), count)
}

func TestEdit_PreservesOtherFields(t *testing.T) {
	db := testutils.NewTestDB(t)

	// Add message with reply
	replyID := int64(5)
	originalMessage := Message{
		MessageID: 1,
		Chat:      Chat{ID: 123},
		From:      &User{ID: 456, FirstName: "Test", Username: "testuser"},
		Date:      1609459200,
		Text:      "Original",
		ReplyTo:   &Message{MessageID: 5},
	}
	originalJSON, _ := json.Marshal(originalMessage)
	entry := CacheEntry{
		ChatID:    123,
		MessageID: 1,
		ReplyID:   &replyID,
		Date:      1609459200,
		Message:   datatypes.JSON(originalJSON),
	}
	require.NoError(t, db.DB.Create(&entry).Error)

	// Edit the message
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	editor := NewEditCommand(NewService(db.DB), logger)
	editedMessage := EditedMessage{
		MessageID: 1,
		Chat:      Chat{ID: 123},
		Date:      1609459200,
		EditDate:  1609459260,
		Text:      "Edited",
	}
	editedJSON, _ := json.Marshal(editedMessage)

	err := editor.Execute(context.Background(), editedJSON)
	require.NoError(t, err)

	// Verify reply_id is preserved
	var updatedEntry CacheEntry
	err = db.DB.First(&updatedEntry, "chat_id = ? AND message_id = ?", 123, 1).Error
	require.NoError(t, err)

	assert.NotNil(t, updatedEntry.ReplyID)
	assert.Equal(t, replyID, *updatedEntry.ReplyID)
}

func TestEdit_DifferentChatID(t *testing.T) {
	db := testutils.NewTestDB(t)

	// Add message in chat 123
	originalMessage := Message{
		MessageID: 1,
		Chat:      Chat{ID: 123},
		From:      &User{ID: 456, FirstName: "Test"},
		Date:      1609459200,
		Text:      "Original",
	}
	originalJSON, _ := json.Marshal(originalMessage)
	entry := CacheEntry{
		ChatID:    123,
		MessageID: 1,
		Date:      1609459200,
		Message:   datatypes.JSON(originalJSON),
	}
	require.NoError(t, db.DB.Create(&entry).Error)

	// Try to edit message with same ID but different chat
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	editor := NewEditCommand(NewService(db.DB), logger)
	editedMessage := EditedMessage{
		MessageID: 1,
		Chat:      Chat{ID: 456},
		Date:      1609459200,
		EditDate:  1609459260,
		Text:      "Edited",
	}
	editedJSON, _ := json.Marshal(editedMessage)

	err := editor.Execute(context.Background(), editedJSON)
	require.NoError(t, err)

	// Original message should be unchanged
	var originalEntry CacheEntry
	err = db.DB.First(&originalEntry, "chat_id = ? AND message_id = ?", 123, 1).Error
	require.NoError(t, err)

	var storedMessage Message
	err = json.Unmarshal(originalEntry.Message, &storedMessage)
	require.NoError(t, err)
	assert.Equal(t, "Original", storedMessage.Text)
}
