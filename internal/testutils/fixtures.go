package testutils

import (
	"fmt"
	"os"
	"testing"
	"time"
)

// LoadFixture loads a JSON fixture file
func LoadFixture(t *testing.T, filename string) []byte {
	data, err := os.ReadFile(fmt.Sprintf("../../testdata/%s", filename))
	if err != nil {
		// Try alternative path
		data, err = os.ReadFile(fmt.Sprintf("../testdata/%s", filename))
		if err != nil {
			t.Fatalf("Failed to load fixture %s: %v", filename, err)
		}
	}
	return data
}

// WaitForCondition waits for a condition to be met or timeout
func WaitForCondition(t *testing.T, condition func() bool, timeout time.Duration, interval time.Duration) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(interval)
	}
	t.Fatal("Condition not met within timeout")
}
