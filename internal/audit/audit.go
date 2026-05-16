package audit

import (
	"encoding/json"
	"errors"
	"strings"
	"time"
)

var ErrInvalidEvent = errors.New("invalid audit event")

type Event struct {
	OrganizationID string
	UserID         string
	DomainID       string
	Action         string
	TargetType     string
	TargetID       string
	Metadata       map[string]string
	CreatedAt      time.Time
}

func New(action string, targetType string, targetID string, actorUserID string) (Event, error) {
	event := Event{
		Action:     strings.TrimSpace(action),
		TargetType: strings.TrimSpace(targetType),
		TargetID:   strings.TrimSpace(targetID),
		UserID:     strings.TrimSpace(actorUserID),
		Metadata:   map[string]string{},
		CreatedAt:  time.Now().UTC(),
	}
	if event.Action == "" || event.TargetType == "" {
		return Event{}, ErrInvalidEvent
	}
	return event, nil
}

func (event Event) MetadataJSON() ([]byte, error) {
	if event.Metadata == nil {
		return []byte("{}"), nil
	}
	return json.Marshal(event.Metadata)
}
