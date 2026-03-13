package stats

import (
	"testing"
	"time"
)

func TestExtractTopLimit(t *testing.T) {
	tests := []struct {
		name         string
		text         string
		defaultLimit int
		maxLimit     int
		wantLimit    int
		wantErr      bool
	}{
		{
			name:         "no argument uses default",
			text:         "!top",
			defaultLimit: 5,
			maxLimit:     20,
			wantLimit:    5,
		},
		{
			name:         "explicit number",
			text:         "!top 10",
			defaultLimit: 5,
			maxLimit:     20,
			wantLimit:    10,
		},
		{
			name:         "max limit",
			text:         "!top 20",
			defaultLimit: 5,
			maxLimit:     20,
			wantLimit:    20,
		},
		{
			name:         "over max returns error",
			text:         "!top 21",
			defaultLimit: 5,
			maxLimit:     20,
			wantErr:      true,
		},
		{
			name:         "zero returns error",
			text:         "!top 0",
			defaultLimit: 5,
			maxLimit:     20,
			wantErr:      true,
		},
		{
			name:         "negative returns error",
			text:         "!top -1",
			defaultLimit: 5,
			maxLimit:     20,
			wantErr:      true,
		},
		{
			name:         "non-number returns error",
			text:         "!top abc",
			defaultLimit: 5,
			maxLimit:     20,
			wantErr:      true,
		},
		{
			name:         "one is valid",
			text:         "!top 1",
			defaultLimit: 5,
			maxLimit:     20,
			wantLimit:    1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := extractTopLimit(tt.text, tt.defaultLimit, tt.maxLimit)
			if tt.wantErr {
				if err == "" {
					t.Fatal("expected error, got nothing")
				}
				return
			}
			if err != "" {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.wantLimit {
				t.Fatalf("expected limit %d, got %d", tt.wantLimit, got)
			}
		})
	}
}

func TestMondayOf(t *testing.T) {
	tests := []struct {
		name   string
		input  time.Time
		expect time.Time
	}{
		{
			name:   "monday returns same day",
			input:  time.Date(2026, 3, 2, 14, 30, 0, 0, time.UTC), // Mon
			expect: time.Date(2026, 3, 2, 0, 0, 0, 0, time.UTC),
		},
		{
			name:   "wednesday goes back to monday",
			input:  time.Date(2026, 3, 4, 10, 0, 0, 0, time.UTC), // Wed
			expect: time.Date(2026, 3, 2, 0, 0, 0, 0, time.UTC),
		},
		{
			name:   "sunday goes back to monday",
			input:  time.Date(2026, 3, 8, 23, 59, 0, 0, time.UTC), // Sun
			expect: time.Date(2026, 3, 2, 0, 0, 0, 0, time.UTC),
		},
		{
			name:   "saturday goes back to monday",
			input:  time.Date(2026, 3, 7, 12, 0, 0, 0, time.UTC), // Sat
			expect: time.Date(2026, 3, 2, 0, 0, 0, 0, time.UTC),
		},
		{
			name:   "friday goes back to monday",
			input:  time.Date(2026, 3, 6, 8, 0, 0, 0, time.UTC), // Fri
			expect: time.Date(2026, 3, 2, 0, 0, 0, 0, time.UTC),
		},
		{
			name:   "cross month boundary",
			input:  time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC), // Sun Mar 1
			expect: time.Date(2026, 2, 23, 0, 0, 0, 0, time.UTC), // Mon Feb 23
		},
		{
			name:   "non-UTC timezone converts to UTC",
			input:  time.Date(2026, 3, 4, 10, 0, 0, 0, time.FixedZone("CET", 3600)), // Wed CET
			expect: time.Date(2026, 3, 2, 0, 0, 0, 0, time.UTC),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mondayOf(tt.input)
			if !got.Equal(tt.expect) {
				t.Fatalf("mondayOf(%v) = %v, want %v", tt.input, got, tt.expect)
			}
			if got.Location() != time.UTC {
				t.Fatalf("expected UTC location, got %v", got.Location())
			}
		})
	}
}

func TestFormatTopResults(t *testing.T) {
	todayStart := time.Date(2026, 3, 6, 0, 0, 0, 0, time.UTC)
	weekStart := time.Date(2026, 3, 2, 0, 0, 0, 0, time.UTC)
	monthStart := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)

	today := []TopUser{
		{UserName: "alice", MessageCount: 42},
		{UserName: "bob", MessageCount: 10},
	}
	week := []TopUser{
		{UserName: "alice", MessageCount: 200},
	}
	month := []TopUser{
		{UserName: "bob", MessageCount: 500},
		{UserName: "alice", MessageCount: 300},
	}
	total := []TopUser{
		{UserName: "bob", MessageCount: 5000},
		{UserName: "alice", MessageCount: 3000},
	}

	result := formatTopResults(5, todayStart, weekStart, monthStart, today, week, month, total)

	// Check key content
	checks := []string{
		"Top 5 users",
		"Today (06 Mar):",
		"1. alice",
		"42",
		"This week (from 02 Mar):",
		"This month (Mar 2026):",
		"All time:",
		"5000",
	}
	for _, check := range checks {
		if !contains(result, check) {
			t.Errorf("expected output to contain %q, got:\n%s", check, result)
		}
	}
}

func TestFormatTopResultsEmpty(t *testing.T) {
	todayStart := time.Date(2026, 3, 6, 0, 0, 0, 0, time.UTC)
	weekStart := time.Date(2026, 3, 2, 0, 0, 0, 0, time.UTC)
	monthStart := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)

	result := formatTopResults(5, todayStart, weekStart, monthStart, nil, nil, nil, nil)

	if !contains(result, "No messages yet.") {
		t.Errorf("expected 'No messages yet.' in output, got:\n%s", result)
	}
}

func TestFormatTopResultsUnknownUser(t *testing.T) {
	todayStart := time.Date(2026, 3, 6, 0, 0, 0, 0, time.UTC)
	weekStart := time.Date(2026, 3, 2, 0, 0, 0, 0, time.UTC)
	monthStart := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)

	users := []TopUser{{UserName: "", MessageCount: 10}}

	result := formatTopResults(5, todayStart, weekStart, monthStart, users, nil, nil, nil)

	if !contains(result, "(unknown)") {
		t.Errorf("expected '(unknown)' in output, got:\n%s", result)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && containsHelper(s, substr)
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
