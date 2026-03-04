package cache

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"

	"gorm.io/datatypes"
)

func (s *CacheDBSuite) TestEdit_UpdatesExistingMessage() {
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
	s.Require().NoError(s.db.DB.Create(&entry).Error)

	// Now edit the message
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	editor := NewEditCommand(NewService(s.db.DB), logger)
	editedMessage := EditedMessage{
		MessageID: 1,
		Chat:      Chat{ID: 123},
		Date:      1609459200,
		EditDate:  1609459260,
		Text:      "Edited text",
	}
	editedJSON, _ := json.Marshal(editedMessage)

	err := editor.Execute(context.Background(), editedJSON)
	s.Require().NoError(err)

	// Verify the message was updated
	var updatedEntry CacheEntry
	err = s.db.DB.First(&updatedEntry, "chat_id = ? AND message_id = ?", 123, 1).Error
	s.Require().NoError(err)

	var storedMessage Message
	err = json.Unmarshal(updatedEntry.Message, &storedMessage)
	s.Require().NoError(err)
	s.Assert().Equal("Edited text", storedMessage.Text)
}

func (s *CacheDBSuite) TestEdit_NonExistentMessage() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	editor := NewEditCommand(NewService(s.db.DB), logger)

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
	s.Require().NoError(err)

	// Verify no entries exist
	var count int64
	s.db.DB.Model(&CacheEntry{}).Count(&count)
	s.Assert().Equal(int64(0), count)
}

func (s *CacheDBSuite) TestEdit_PreservesOtherFields() {
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
	s.Require().NoError(s.db.DB.Create(&entry).Error)

	// Edit the message
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	editor := NewEditCommand(NewService(s.db.DB), logger)
	editedMessage := EditedMessage{
		MessageID: 1,
		Chat:      Chat{ID: 123},
		Date:      1609459200,
		EditDate:  1609459260,
		Text:      "Edited",
	}
	editedJSON, _ := json.Marshal(editedMessage)

	err := editor.Execute(context.Background(), editedJSON)
	s.Require().NoError(err)

	// Verify reply_id is preserved
	var updatedEntry CacheEntry
	err = s.db.DB.First(&updatedEntry, "chat_id = ? AND message_id = ?", 123, 1).Error
	s.Require().NoError(err)

	s.Assert().NotNil(updatedEntry.ReplyID)
	s.Assert().Equal(replyID, *updatedEntry.ReplyID)
}

func (s *CacheDBSuite) TestEdit_DifferentChatID() {
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
	s.Require().NoError(s.db.DB.Create(&entry).Error)

	// Try to edit message with same ID but different chat
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	editor := NewEditCommand(NewService(s.db.DB), logger)
	editedMessage := EditedMessage{
		MessageID: 1,
		Chat:      Chat{ID: 456},
		Date:      1609459200,
		EditDate:  1609459260,
		Text:      "Edited",
	}
	editedJSON, _ := json.Marshal(editedMessage)

	err := editor.Execute(context.Background(), editedJSON)
	s.Require().NoError(err)

	// Original message should be unchanged
	var originalEntry CacheEntry
	err = s.db.DB.First(&originalEntry, "chat_id = ? AND message_id = ?", 123, 1).Error
	s.Require().NoError(err)

	var storedMessage Message
	err = json.Unmarshal(originalEntry.Message, &storedMessage)
	s.Require().NoError(err)
	s.Assert().Equal("Original", storedMessage.Text)
}
