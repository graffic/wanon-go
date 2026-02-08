package quotes

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/datatypes"
)

func TestRenderer_Render(t *testing.T) {
	tests := []struct {
		name        string
		quote       *Quote
		includeID   bool
		wantText    string
		wantCount   int
		wantErr     bool
		errContains string
	}{
		{
			name:      "single message",
			quote:     createTestQuote(1, []testMessage{{FirstName: "John", Text: "Hello world"}}),
			includeID: false,
			wantText:  "John: Hello world",
			wantCount: 1,
			wantErr:   false,
		},
		{
			name: "multiple messages",
			quote: createTestQuote(1, []testMessage{
				{FirstName: "Alice", Text: "First message"},
				{FirstName: "Bob", Text: "Second message"},
				{FirstName: "Charlie", Text: "Third message"},
			}),
			includeID: false,
			wantText:  "Alice: First message\nBob: Second message\nCharlie: Third message",
			wantCount: 3,
			wantErr:   false,
		},
		{
			name:      "with quote ID",
			quote:     createTestQuote(42, []testMessage{{FirstName: "John", Text: "Hello"}}),
			includeID: true,
			wantText:  "#42\nJohn: Hello",
			wantCount: 1,
			wantErr:   false,
		},
		{
			name:        "nil quote",
			quote:       nil,
			wantErr:     true,
			errContains: "cannot render nil quote",
		},
		{
			name:        "empty entries",
			quote:       createTestQuote(1, []testMessage{}),
			wantErr:     true,
			errContains: "cannot render quote with no entries",
		},
		{
			name:      "empty text",
			quote:     createTestQuote(1, []testMessage{{FirstName: "John", Text: ""}}),
			wantText:  "John: (no text)",
			wantCount: 1,
			wantErr:   false,
		},
		{
			name:      "no text field",
			quote:     createTestQuoteWithRawMessage(1, map[string]interface{}{"from": map[string]interface{}{"first_name": "John"}}),
			wantText:  "John: (no text)",
			wantCount: 1,
			wantErr:   false,
		},
		{
			name:      "no from field",
			quote:     createTestQuoteWithRawMessage(1, map[string]interface{}{"text": "Hello world"}),
			wantText:  "Unknown: Hello world",
			wantCount: 1,
			wantErr:   false,
		},
		{
			name:      "no first name - username fallback",
			quote:     createTestQuote(1, []testMessage{{Username: "john_doe", Text: "Hello"}}),
			wantText:  "@john_doe: Hello",
			wantCount: 1,
			wantErr:   false,
		},
		{
			name:      "first name over username",
			quote:     createTestQuote(1, []testMessage{{FirstName: "John", Username: "john_doe", Text: "Hello"}}),
			wantText:  "John: Hello",
			wantCount: 1,
			wantErr:   false,
		},
		{
			name:      "full name with last name",
			quote:     createTestQuote(1, []testMessage{{FirstName: "John", LastName: "Doe", Text: "Hello"}}),
			wantText:  "John Doe: Hello",
			wantCount: 1,
			wantErr:   false,
		},
		{
			name:      "last name only",
			quote:     createTestQuote(1, []testMessage{{LastName: "Doe", Text: "Hello"}}),
			wantText:  "Doe: Hello",
			wantCount: 1,
			wantErr:   false,
		},
		{
			name:      "completely empty from",
			quote:     createTestQuote(1, []testMessage{{Text: "Hello"}}),
			wantText:  "Unknown: Hello",
			wantCount: 1,
			wantErr:   false,
		},
	}

	renderer := NewRenderer()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := renderer.Render(RenderOptions{
				Quote:     tt.quote,
				IncludeID: tt.includeID,
			})

			if tt.wantErr {
				require.Error(t, err)
				if tt.errContains != "" {
					assert.Contains(t, err.Error(), tt.errContains)
				}
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.wantText, result.Text)
				assert.Equal(t, tt.wantCount, result.EntryCount)
			}
		})
	}
}

func TestRenderer_RenderSimple(t *testing.T) {
	renderer := NewRenderer()
	quote := createTestQuote(1, []testMessage{{FirstName: "John", Text: "Hello world"}})

	result, err := renderer.RenderSimple(quote)
	require.NoError(t, err)
	assert.Equal(t, "John: Hello world", result)
}

func TestRenderer_RenderSimple_Error(t *testing.T) {
	renderer := NewRenderer()

	_, err := renderer.RenderSimple(nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot render nil quote")
}

func TestRenderer_RenderWithDate(t *testing.T) {
	tests := []struct {
		name     string
		quote    *Quote
		wantText string
		wantErr  bool
	}{
		{
			name:     "with date",
			quote:    createTestQuoteWithDate(42, []testMessage{{FirstName: "John", Text: "Hello"}}, 1609459200), // 2021-01-01 00:00:00 UTC
			wantText: "#42\nJohn: Hello\n📅 2021-01-01 00:00",                                                     // UTC time
			wantErr:  false,
		},
		{
			name:     "without date",
			quote:    createTestQuote(42, []testMessage{{FirstName: "John", Text: "Hello"}}),
			wantText: "#42\nJohn: Hello",
			wantErr:  false,
		},
		{
			name:    "nil quote",
			quote:   nil,
			wantErr: true,
		},
	}

	renderer := NewRenderer()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := renderer.RenderWithDate(tt.quote)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.wantText, result)
			}
		})
	}
}

func TestRenderer_buildAuthorName(t *testing.T) {
	tests := []struct {
		firstName string
		lastName  string
		username  string
		expected  string
	}{
		{"John", "", "", "John"},
		{"", "Doe", "", "Doe"},
		{"John", "Doe", "", "John Doe"},
		{"", "", "johndoe", "@johndoe"},
		{"John", "", "johndoe", "John"},
		{"", "", "", "Unknown"},
	}

	renderer := NewRenderer()

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := renderer.buildAuthorName(tt.firstName, tt.lastName, tt.username)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// Test helpers

type testMessage struct {
	FirstName string
	LastName  string
	Username  string
	Text      string
	Date      int64
}

func createTestQuote(id uint, messages []testMessage) *Quote {
	entries := make([]QuoteEntry, len(messages))
	for i, msg := range messages {
		data := map[string]interface{}{
			"from": map[string]interface{}{
				"first_name": msg.FirstName,
				"last_name":  msg.LastName,
				"username":   msg.Username,
			},
			"text": msg.Text,
			"date": msg.Date,
		}
		jsonData, _ := json.Marshal(data)
		entries[i] = QuoteEntry{
			Order:   i,
			Message: datatypes.JSON(jsonData),
		}
	}
	return &Quote{
		ID:      id,
		Entries: entries,
	}
}

func createTestQuoteWithDate(id uint, messages []testMessage, date int64) *Quote {
	entries := make([]QuoteEntry, len(messages))
	for i, msg := range messages {
		data := map[string]interface{}{
			"from": map[string]interface{}{
				"first_name": msg.FirstName,
				"last_name":  msg.LastName,
				"username":   msg.Username,
			},
			"text": msg.Text,
			"date": date,
		}
		jsonData, _ := json.Marshal(data)
		entries[i] = QuoteEntry{
			Order:   i,
			Message: datatypes.JSON(jsonData),
		}
	}
	return &Quote{
		ID:      id,
		Entries: entries,
	}
}

func createTestQuoteWithRawMessage(id uint, data map[string]interface{}) *Quote {
	jsonData, _ := json.Marshal(data)
	return &Quote{
		ID: id,
		Entries: []QuoteEntry{
			{Order: 0, Message: datatypes.JSON(jsonData)},
		},
	}
}
