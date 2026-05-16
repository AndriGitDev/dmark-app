package ingest

import (
	"crypto/sha256"
	"encoding/hex"
	"time"
)

type DeadLetter struct {
	ID          string
	SourceFile  string
	ContentHash string
	Reason      string
	Detail      string
	CreatedAt   time.Time
}

func NewDeadLetter(sourceFile string, payload []byte, reason string, detail string, now time.Time) DeadLetter {
	sum := sha256.Sum256(payload)
	if now.IsZero() {
		now = time.Now().UTC()
	}
	return DeadLetter{
		SourceFile:  sourceFile,
		ContentHash: hex.EncodeToString(sum[:]),
		Reason:      reason,
		Detail:      detail,
		CreatedAt:   now.UTC(),
	}
}

func (letter DeadLetter) ReviewLabel() string {
	if letter.SourceFile == "" {
		return letter.Reason + " " + letter.ContentHash
	}
	return letter.Reason + " " + letter.SourceFile
}
