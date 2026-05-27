package salesaudit

import (
	"fmt"
	"strings"
	"time"
)

type ClaimStatus string

const (
	ClaimMagicLinkSent ClaimStatus = "magic_link_sent"
)

type Claim struct {
	ID            string
	Domain        string
	Email         string
	Grade         string
	Risk          RiskLevel
	Safe          bool
	ProofEligible bool
	Status        ClaimStatus
	CreatedAt     time.Time
}

func NewClaim(domain string, email string, audit Result, now time.Time) (Claim, error) {
	domain = strings.TrimSpace(strings.ToLower(domain))
	email = strings.TrimSpace(strings.ToLower(email))
	if domain == "" || email == "" {
		return Claim{}, fmt.Errorf("sales audit claim requires domain and email")
	}
	if audit.Grade == "" || audit.Risk == "" {
		return Claim{}, fmt.Errorf("sales audit claim requires audit result")
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	return Claim{
		Domain:        domain,
		Email:         email,
		Grade:         audit.Grade,
		Risk:          audit.Risk,
		Safe:          audit.Safe,
		ProofEligible: audit.ProofEligible,
		Status:        ClaimMagicLinkSent,
		CreatedAt:     now.UTC(),
	}, nil
}
