package admin

import (
	"fmt"
	"strings"
	"time"
)

type CaseStatus string

const (
	CaseOpen     CaseStatus = "open"
	CaseResolved CaseStatus = "resolved"
)

type CaseKind string

const (
	CaseSupport CaseKind = "support"
	CaseAbuse   CaseKind = "abuse"
	CaseBilling CaseKind = "billing"
)

type Case struct {
	ID         string
	Kind       CaseKind
	Status     CaseStatus
	Subject    string
	UserID     string
	DomainID   string
	AssignedTo string
	Notes      []Note
	CreatedAt  time.Time
	ResolvedAt time.Time
}

type Note struct {
	AuthorID  string
	Body      string
	CreatedAt time.Time
}

func NewCase(kind CaseKind, subject string, userID string, domainID string, now time.Time) (Case, error) {
	subject = strings.TrimSpace(subject)
	if kind == "" || subject == "" {
		return Case{}, fmt.Errorf("admin case requires kind and subject")
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	return Case{Kind: kind, Status: CaseOpen, Subject: subject, UserID: userID, DomainID: domainID, CreatedAt: now.UTC()}, nil
}

func (c Case) AddNote(authorID string, body string, now time.Time) (Case, error) {
	body = strings.TrimSpace(body)
	if body == "" {
		return Case{}, fmt.Errorf("admin note body is required")
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	c.Notes = append(c.Notes, Note{AuthorID: authorID, Body: body, CreatedAt: now.UTC()})
	return c, nil
}

func (c Case) Resolve(now time.Time) Case {
	if now.IsZero() {
		now = time.Now().UTC()
	}
	c.Status = CaseResolved
	c.ResolvedAt = now.UTC()
	return c
}

type AbuseAction string

const (
	AbuseActionRateLimit AbuseAction = "rate_limit"
	AbuseActionBlock     AbuseAction = "block"
	AbuseActionReview    AbuseAction = "review"
)

type AbuseDecision struct {
	ID         string
	Identifier string
	Action     AbuseAction
	Reason     string
	ExpiresAt  time.Time
	CreatedAt  time.Time
}

func NewAbuseDecision(identifier string, action AbuseAction, reason string, duration time.Duration, now time.Time) (AbuseDecision, error) {
	identifier = strings.TrimSpace(identifier)
	reason = strings.TrimSpace(reason)
	if identifier == "" || action == "" || reason == "" {
		return AbuseDecision{}, fmt.Errorf("abuse decision requires identifier, action, and reason")
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	decision := AbuseDecision{Identifier: identifier, Action: action, Reason: reason}
	if duration > 0 {
		decision.ExpiresAt = now.UTC().Add(duration)
	}
	return decision, nil
}
