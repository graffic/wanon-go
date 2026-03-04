package stats

import (
	"testing"
	"time"
)

func TestBucketTimeUTC(t *testing.T) {
	loc := time.FixedZone("PST", -8*60*60)
	input := time.Date(2026, 3, 5, 12, 34, 56, 123, loc)

	bucket := bucketTime(input)

	expected := time.Date(2026, 3, 5, 20, 0, 0, 0, time.UTC)
	if !bucket.Equal(expected) {
		t.Fatalf("expected bucket %v, got %v", expected, bucket)
	}
}
