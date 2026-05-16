package ingest

import (
	"strings"
	"testing"
	"time"
)

func TestNewDeadLetter(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC)
	letter := NewDeadLetter("bad.xml", []byte("<bad>"), "parse_error", "invalid XML", now)
	if letter.SourceFile != "bad.xml" || letter.ContentHash == "" || letter.CreatedAt != now {
		t.Fatalf("unexpected dead letter: %+v", letter)
	}
	if !strings.Contains(letter.ReviewLabel(), "bad.xml") {
		t.Fatalf("ReviewLabel = %q, want source file", letter.ReviewLabel())
	}
}
