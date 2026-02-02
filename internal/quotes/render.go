package quotes

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// Render formats quotes as readable text.
// This ports the Quotes.Render.render functionality from Elixir.

type Renderer struct{}

// NewRenderer creates a new quote renderer
func NewRenderer() *Renderer {
	return &Renderer{}
}

// RenderOptions contains options for rendering a quote
type RenderOptions struct {
	Quote      *Quote
	IncludeID  bool
}

// RenderResult contains the rendered quote text and metadata
type RenderResult struct {
	Text       string
	EntryCount int
}

// Render formats a quote as readable text.
// Each entry is formatted with author name and message text.
func (r *Renderer) Render(opts RenderOptions) (*RenderResult, error) {
	if opts.Quote == nil {
		return nil, fmt.Errorf("cannot render nil quote")
	}

	if len(opts.Quote.Entries) == 0 {
		return nil, fmt.Errorf("cannot render quote with no entries")
	}

	var parts []string

	// Render each entry
	for _, entry := range opts.Quote.Entries {
		rendered, err := r.renderEntry(entry)
		if err != nil {
			return nil, fmt.Errorf("failed to render entry %d: %w", entry.Order, err)
		}
		parts = append(parts, rendered)
	}

	// Join entries with newlines
	text := strings.Join(parts, "\n")

	// Optionally include quote ID
	if opts.IncludeID {
		text = fmt.Sprintf("#%d\n%s", opts.Quote.ID, text)
	}

	return &RenderResult{
		Text:       text,
		EntryCount: len(opts.Quote.Entries),
	}, nil
}

// renderEntry formats a single quote entry as text
func (r *Renderer) renderEntry(entry QuoteEntry) (string, error) {
	// Extract message data from JSON
	var msgData struct {
		Text string `json:"text"`
		From struct {
			FirstName    string `json:"first_name"`
			LastName     string `json:"last_name"`
			Username     string `json:"username"`
			ID           int64  `json:"id"`
			IsBot        bool   `json:"is_bot"`
			LanguageCode string `json:"language_code"`
		} `json:"from"`
		Date int64 `json:"date"`
	}

	if err := json.Unmarshal(entry.Message, &msgData); err != nil {
		return "", fmt.Errorf("failed to unmarshal message: %w", err)
	}

	// Build author name
	authorName := r.buildAuthorName(msgData.From.FirstName, msgData.From.LastName, msgData.From.Username)

	// Format the entry
	// Format: "<Author Name>: <message text>"
	if msgData.Text == "" {
		msgData.Text = "(no text)"
	}

	return fmt.Sprintf("%s: %s", authorName, msgData.Text), nil
}

// buildAuthorName builds a display name from user info
func (r *Renderer) buildAuthorName(firstName, lastName, username string) string {
	var parts []string
	
	if firstName != "" {
		parts = append(parts, firstName)
	}
	if lastName != "" {
		parts = append(parts, lastName)
	}
	
	name := strings.Join(parts, " ")
	
	// If no name available, use username
	if name == "" && username != "" {
		name = "@" + username
	}
	
	// Fallback
	if name == "" {
		name = "Unknown"
	}
	
	return name
}

// RenderSimple renders a quote in a simple format (just the text)
func (r *Renderer) RenderSimple(quote *Quote) (string, error) {
	result, err := r.Render(RenderOptions{Quote: quote})
	if err != nil {
		return "", err
	}
	return result.Text, nil
}

// RenderWithDate renders a quote including the date of the first message
func (r *Renderer) RenderWithDate(quote *Quote) (string, error) {
	result, err := r.Render(RenderOptions{Quote: quote, IncludeID: true})
	if err != nil {
		return "", err
	}

	// Try to extract date from first entry
	if len(quote.Entries) > 0 {
		var msgData struct {
			Date int64 `json:"date"`
		}
		if err := json.Unmarshal(quote.Entries[0].Message, &msgData); err == nil && msgData.Date > 0 {
			msgTime := time.Unix(msgData.Date, 0)
			dateStr := msgTime.Format("2006-01-02 15:04")
			result.Text = fmt.Sprintf("%s\n📅 %s", result.Text, dateStr)
		}
	}

	return result.Text, nil
}
