package jobs

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"time"
)

type DeadLetter struct {
	Kind           string
	IdempotencyKey string
	ArgsJSON       []byte
	LastError      string
	Attempts       int
	FailedAt       time.Time
}

func NewDeadLetter(kind string, idempotencyKey string, args any, attempts int, err error, failedAt time.Time) (DeadLetter, error) {
	argsJSON, marshalErr := json.Marshal(args)
	if marshalErr != nil {
		return DeadLetter{}, marshalErr
	}
	lastError := ""
	if err != nil {
		lastError = err.Error()
	}
	return DeadLetter{
		Kind:           strings.TrimSpace(kind),
		IdempotencyKey: strings.TrimSpace(idempotencyKey),
		ArgsJSON:       argsJSON,
		LastError:      strings.TrimSpace(lastError),
		Attempts:       attempts,
		FailedAt:       failedAt.UTC(),
	}, nil
}

func (letter DeadLetter) ReviewKey() string {
	sum := sha256.Sum256([]byte(letter.Kind + "\x00" + letter.IdempotencyKey + "\x00" + string(letter.ArgsJSON)))
	return hex.EncodeToString(sum[:])
}
