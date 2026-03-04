package cache

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
)

func (s *CacheDBSuite) TestAdd_StoresMessageInCache() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	adder := NewAddCommand(NewService(s.db.DB), logger)

	message := Message{
		MessageID: 1,
		Chat:      Chat{ID: 123},
		From:      &User{ID: 456, FirstName: "Test"},
		Date:      1609459200,
		Text:      "Test message",
	}
	messageJSON, _ := json.Marshal(message)

	err := adder.Execute(context.Background(), messageJSON)
	s.Require().NoError(err)

	// Verify entry was created
	var entry CacheEntry
	err = s.db.DB.First(&entry, "chat_id = ? AND message_id = ?", 123, 1).Error
	s.Require().NoError(err)

	s.Assert().Equal(int64(123), entry.ChatID)
	s.Assert().Equal(int64(1), entry.MessageID)
	s.Assert().Equal(int64(1609459200), entry.Date)
	s.Assert().NotNil(entry.Message)
}

func (s *CacheDBSuite) TestAdd_StoresReplyID() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	adder := NewAddCommand(NewService(s.db.DB), logger)

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
	s.Require().NoError(err)

	var entry CacheEntry
	err = s.db.DB.First(&entry, "chat_id = ? AND message_id = ?", 123, 1).Error
	s.Require().NoError(err)

	s.Assert().NotNil(entry.ReplyID)
	s.Assert().Equal(replyID, *entry.ReplyID)
}

func (s *CacheDBSuite) TestAdd_StoresFullMessageJSON() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	adder := NewAddCommand(NewService(s.db.DB), logger)

	message := Message{
		MessageID: 1,
		Chat:      Chat{ID: 123},
		From:      &User{ID: 456, FirstName: "Test", Username: "testuser"},
		Date:      1609459200,
		Text:      "Test message",
	}
	messageJSON, _ := json.Marshal(message)

	err := adder.Execute(context.Background(), messageJSON)
	s.Require().NoError(err)

	var entry CacheEntry
	err = s.db.DB.First(&entry, "chat_id = ? AND message_id = ?", 123, 1).Error
	s.Require().NoError(err)

	// Verify message JSON
	var storedMessage Message
	err = json.Unmarshal(entry.Message, &storedMessage)
	s.Require().NoError(err)

	s.Assert().Equal(message.MessageID, storedMessage.MessageID)
	s.Assert().Equal(message.Text, storedMessage.Text)
}

func (s *CacheDBSuite) TestAdd_DuplicateMessageUpdates() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	adder := NewAddCommand(NewService(s.db.DB), logger)

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
	s.Require().NoError(err)

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
	s.Require().NoError(err)

	// Verify only one entry exists with updated content
	var entries []CacheEntry
	err = s.db.DB.Where("chat_id = ? AND message_id = ?", 123, 1).Find(&entries).Error
	s.Require().NoError(err)
	s.Assert().Len(entries, 1)

	var storedMessage Message
	err = json.Unmarshal(entries[0].Message, &storedMessage)
	s.Require().NoError(err)
	s.Assert().Equal("Updated", storedMessage.Text)
}
