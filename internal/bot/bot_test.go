package bot

import (
	"context"
	"testing"
	"time"

	"github.com/go-telegram/bot/models"
	"github.com/graffic/wanon-go/internal/telegram"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockTelegramClient is a mock for the telegram.Client interface
type MockTelegramClient struct {
	mock.Mock
}

func (m *MockTelegramClient) GetMe(ctx context.Context) (*models.User, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.User), args.Error(1)
}

func (m *MockTelegramClient) GetUpdates(ctx context.Context, offset int, limit int, timeout int) ([]models.Update, error) {
	args := m.Called(ctx, offset, limit, timeout)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.Update), args.Error(1)
}

func (m *MockTelegramClient) SendMessage(ctx context.Context, chatID int64, text string, replyToMessageID *int64) (*models.Message, error) {
	args := m.Called(ctx, chatID, text, replyToMessageID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Message), args.Error(1)
}

func (m *MockTelegramClient) SendText(ctx context.Context, chatID int64, text string) (*models.Message, error) {
	args := m.Called(ctx, chatID, text)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Message), args.Error(1)
}

func (m *MockTelegramClient) ReplyToMessage(ctx context.Context, chatID int64, messageID int64, text string) (*models.Message, error) {
	args := m.Called(ctx, chatID, messageID, text)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Message), args.Error(1)
}

func (m *MockTelegramClient) SetWebhook(ctx context.Context, url string) error {
	args := m.Called(ctx, url)
	return args.Error(0)
}

func (m *MockTelegramClient) DeleteWebhook(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockTelegramClient) GetChat(ctx context.Context, chatID int64) (*models.ChatFullInfo, error) {
	args := m.Called(ctx, chatID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.ChatFullInfo), args.Error(1)
}

func (m *MockTelegramClient) GetChatAdministrators(ctx context.Context, chatID int64) ([]models.ChatMember, error) {
	args := m.Called(ctx, chatID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]models.ChatMember), args.Error(1)
}

// Ensure MockTelegramClient implements telegram.Client
var _ telegram.Client = (*MockTelegramClient)(nil)

// MockCommand is a mock command for testing
type MockCommand struct {
	mock.Mock
}

func (m *MockCommand) Execute(ctx context.Context, msg *models.Message) error {
	args := m.Called(ctx, msg)
	return args.Error(0)
}

func TestDispatcher_isChatAllowed_AllowedChat(t *testing.T) {
	allowedChats := []int64{-1001234567890}
	updatesCh := make(chan []models.Update)
	dispatcher := NewDispatcher(updatesCh, allowedChats)

	assert.True(t, dispatcher.isChatAllowed(-1001234567890))
}

func TestDispatcher_isChatAllowed_DisallowedChat(t *testing.T) {
	allowedChats := []int64{-1001234567890}
	updatesCh := make(chan []models.Update)
	dispatcher := NewDispatcher(updatesCh, allowedChats)

	assert.False(t, dispatcher.isChatAllowed(-1009999999999))
}

func TestDispatcher_isChatAllowed_MultipleAllowedChats(t *testing.T) {
	allowedChats := []int64{-1001234567890, -1009999999999}
	updatesCh := make(chan []models.Update)
	dispatcher := NewDispatcher(updatesCh, allowedChats)

	tests := []struct {
		chatID  int64
		allowed bool
	}{
		{-1001234567890, true},
		{-1009999999999, true},
		{-1001111111111, false},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.allowed, dispatcher.isChatAllowed(tt.chatID), "chat %d", tt.chatID)
	}
}

func TestDispatcher_isChatAllowed_NoWhitelist(t *testing.T) {
	updatesCh := make(chan []models.Update)
	dispatcher := NewDispatcher(updatesCh, nil)

	// If no whitelist is configured, all chats should be allowed
	assert.True(t, dispatcher.isChatAllowed(-1001234567890))
	assert.True(t, dispatcher.isChatAllowed(-1009999999999))
}

func TestExtractCommand(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		expected string
	}{
		{
			name:     "simple command",
			text:     "/start",
			expected: "start",
		},
		{
			name:     "command with args",
			text:     "/addquote hello world",
			expected: "addquote",
		},
		{
			name:     "command with bot username",
			text:     "/start@mybot",
			expected: "start",
		},
		{
			name:     "not a command",
			text:     "hello world",
			expected: "",
		},
		{
			name:     "empty string",
			text:     "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractCommand(tt.text)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestDispatcher_processUpdates(t *testing.T) {
	mockCmd := new(MockCommand)
	updatesCh := make(chan []models.Update, 1)
	dispatcher := NewDispatcher(updatesCh, []int64{-100123})
	dispatcher.Register("test", mockCmd)

	updates := []models.Update{
		{
			ID: 1,
			Message: &models.Message{
				ID:   1,
				Chat: models.Chat{ID: -100123},
				Text: "/test",
			},
		},
	}

	mockCmd.On("Execute", mock.Anything, updates[0].Message).Return(nil)

	dispatcher.processUpdates(context.Background(), updates)

	mockCmd.AssertExpectations(t)
}

func TestDispatcher_processUpdates_UnknownCommand(t *testing.T) {
	updatesCh := make(chan []models.Update, 1)
	dispatcher := NewDispatcher(updatesCh, []int64{-100123})

	updates := []models.Update{
		{
			ID: 1,
			Message: &models.Message{
				ID:   1,
				Chat: models.Chat{ID: -100123},
				Text: "/unknown",
			},
		},
	}

	// Should not panic or error - just skip unknown commands
	dispatcher.processUpdates(context.Background(), updates)
}

func TestDispatcher_processUpdates_DisallowedChat(t *testing.T) {
	mockCmd := new(MockCommand)
	updatesCh := make(chan []models.Update, 1)
	dispatcher := NewDispatcher(updatesCh, []int64{-100123})
	dispatcher.Register("test", mockCmd)

	updates := []models.Update{
		{
			ID: 1,
			Message: &models.Message{
				ID:   1,
				Chat: models.Chat{ID: -100999}, // Not allowed
				Text: "/test",
			},
		},
	}

	// Command should not be called for disallowed chat
	dispatcher.processUpdates(context.Background(), updates)

	mockCmd.AssertNotCalled(t, "Execute")
}

func TestDispatcher_processUpdates_EditedMessage(t *testing.T) {
	mockCmd := new(MockCommand)
	updatesCh := make(chan []models.Update, 1)
	dispatcher := NewDispatcher(updatesCh, []int64{-100123})
	dispatcher.Register("test", mockCmd)

	updates := []models.Update{
		{
			ID: 1,
			EditedMessage: &models.Message{
				ID:   1,
				Chat: models.Chat{ID: -100123},
				Text: "/test",
			},
		},
	}

	mockCmd.On("Execute", mock.Anything, updates[0].EditedMessage).Return(nil)

	dispatcher.processUpdates(context.Background(), updates)

	mockCmd.AssertExpectations(t)
}

func TestUpdates_lastUpdateID(t *testing.T) {
	tests := []struct {
		name     string
		updates  []models.Update
		expected int
	}{
		{
			name:     "empty updates",
			updates:  []models.Update{},
			expected: 0,
		},
		{
			name: "single update",
			updates: []models.Update{
				{ID: 5},
			},
			expected: 5,
		},
		{
			name: "multiple updates",
			updates: []models.Update{
				{ID: 1},
				{ID: 5},
				{ID: 3},
			},
			expected: 5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := lastUpdateID(tt.updates)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestUpdates_Start_ContextCancellation(t *testing.T) {
	mockClient := new(MockTelegramClient)
	updatesCh := make(chan []models.Update, 10)

	// Return empty updates to avoid blocking
	mockClient.On("GetUpdates", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return([]models.Update{}, nil)

	updates := NewUpdates(mockClient, updatesCh)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	err := updates.Start(ctx)
	require.Error(t, err)
	assert.Equal(t, context.Canceled, err)
}

func TestUpdates_Start_ReceivesUpdates(t *testing.T) {
	mockClient := new(MockTelegramClient)
	updatesCh := make(chan []models.Update, 10)

	// First call returns updates, subsequent calls return empty
	mockClient.On("GetUpdates", mock.Anything, 0, mock.Anything, mock.Anything).
		Return([]models.Update{
			{ID: 1, Message: &models.Message{ID: 1, Chat: models.Chat{ID: 123}}},
		}, nil).Once()
	mockClient.On("GetUpdates", mock.Anything, 2, mock.Anything, mock.Anything).
		Return([]models.Update{}, nil)

	updates := NewUpdates(mockClient, updatesCh)

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	go updates.Start(ctx)

	// Wait for updates to be received
	select {
	case received := <-updatesCh:
		require.Len(t, received, 1)
		assert.Equal(t, int64(1), received[0].ID)
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for updates")
	}
}
