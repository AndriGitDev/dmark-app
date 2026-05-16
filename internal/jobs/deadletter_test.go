package jobs

import (
	"errors"
	"testing"
	"time"
)

func TestNewDeadLetter(t *testing.T) {
	t.Parallel()

	failedAt := time.Date(2026, 5, 16, 12, 0, 0, 0, time.UTC)
	letter, err := NewDeadLetter(ScanDomainKind, "key-1", ScanDomainArgs{Domain: "example.com"}, 5, errors.New("resolver timeout"), failedAt)
	if err != nil {
		t.Fatalf("NewDeadLetter returned error: %v", err)
	}
	if letter.Kind != ScanDomainKind || letter.IdempotencyKey != "key-1" || letter.LastError != "resolver timeout" || letter.Attempts != 5 {
		t.Fatalf("unexpected dead letter: %+v", letter)
	}
	if len(letter.ArgsJSON) == 0 || letter.ReviewKey() == "" {
		t.Fatalf("expected args JSON and review key: %+v", letter)
	}
}

func TestDeadLetterReviewKeyStable(t *testing.T) {
	t.Parallel()

	first, err := NewDeadLetter("kind", "key", map[string]string{"domain": "example.com"}, 1, nil, time.Now())
	if err != nil {
		t.Fatalf("NewDeadLetter returned error: %v", err)
	}
	second, err := NewDeadLetter("kind", "key", map[string]string{"domain": "example.com"}, 2, errors.New("different"), time.Now())
	if err != nil {
		t.Fatalf("NewDeadLetter returned error: %v", err)
	}
	if first.ReviewKey() != second.ReviewKey() {
		t.Fatalf("ReviewKey should ignore attempt/error/time for same job payload")
	}
}
