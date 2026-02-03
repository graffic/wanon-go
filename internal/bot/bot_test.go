package bot

import (
	"context"
	"testing"

	"github.com/go-telegram/bot/models"
	"github.com/stretchr/testify/assert"
)

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

// extractCommand extracts the command name from message text
// Returns empty string if no command is found
func extractCommand(text string) string {
	if len(text) == 0 || text[0] != '/' {
		return ""
	}

	// Find the end of the command (space or end of string)
	end := len(text)
	for i, c := range text {
		if c == ' ' {
			end = i
			break
		}
	}

	// Extract command without the leading '/'
	cmd := text[1:end]

	// Handle commands with bot username (e.g., /start@mybot)
	for i, c := range cmd {
		if c == '@' {
			cmd = cmd[:i]
			break
		}
	}

	return cmd
}

func TestRegistry_RegisterAndGet(t *testing.T) {
	registry := NewRegistry()

	// Create a simple command
	cmd := CommandFunc(func(ctx context.Context, msg *models.Message) error {
		return nil
	})

	// Register command
	registry.Register("test", cmd)

	// Verify it exists
	assert.True(t, registry.Has("test"))

	// Retrieve command
	retrieved, ok := registry.Get("test")
	assert.True(t, ok)
	assert.NotNil(t, retrieved)
}

func TestRegistry_List(t *testing.T) {
	registry := NewRegistry()

	// Register multiple commands
	cmd := CommandFunc(func(ctx context.Context, msg *models.Message) error {
		return nil
	})

	registry.Register("cmd1", cmd)
	registry.Register("cmd2", cmd)
	registry.Register("cmd3", cmd)

	// List commands
	list := registry.List()
	assert.Len(t, list, 3)
	assert.Contains(t, list, "cmd1")
	assert.Contains(t, list, "cmd2")
	assert.Contains(t, list, "cmd3")
}
