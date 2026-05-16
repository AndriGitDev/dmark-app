package notifier

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"time"
)

type DispatchStatus string

const (
	DispatchPending DispatchStatus = "pending"
	DispatchSent    DispatchStatus = "sent"
	DispatchFailed  DispatchStatus = "failed"
	DispatchSkipped DispatchStatus = "skipped"
)

type DispatchRecord struct {
	ID                string
	DomainID          string
	DriftEventID      string
	Channel           string
	Recipient         string
	Subject           string
	Status            DispatchStatus
	ProviderMessageID string
	Error             string
	CreatedAt         time.Time
}

func NewDispatchRecord(domainID string, driftEventID string, channel string, recipient string, subject string, now time.Time) DispatchRecord {
	return DispatchRecord{
		DomainID:     strings.TrimSpace(domainID),
		DriftEventID: strings.TrimSpace(driftEventID),
		Channel:      strings.ToLower(strings.TrimSpace(channel)),
		Recipient:    strings.ToLower(strings.TrimSpace(recipient)),
		Subject:      strings.TrimSpace(subject),
		Status:       DispatchPending,
		CreatedAt:    now.UTC(),
	}
}

func (record DispatchRecord) IdempotencyKey() string {
	sum := sha256.Sum256([]byte(record.DriftEventID + "\x00" + record.Channel + "\x00" + record.Recipient))
	return hex.EncodeToString(sum[:])
}

func (record DispatchRecord) Sent(providerMessageID string) DispatchRecord {
	record.Status = DispatchSent
	record.ProviderMessageID = strings.TrimSpace(providerMessageID)
	record.Error = ""
	return record
}

func (record DispatchRecord) Failed(err error) DispatchRecord {
	record.Status = DispatchFailed
	if err != nil {
		record.Error = err.Error()
	}
	return record
}

func (record DispatchRecord) Skipped(reason string) DispatchRecord {
	record.Status = DispatchSkipped
	record.Error = strings.TrimSpace(reason)
	return record
}
