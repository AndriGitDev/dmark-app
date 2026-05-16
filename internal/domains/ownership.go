package domains

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"strings"
)

const OwnershipTXTName = "_meil-verify"

type OwnershipChallenge struct {
	Domain string
	Name   string
	Value  string
}

func NewOwnershipChallenge(domain string) (OwnershipChallenge, error) {
	raw := make([]byte, 24)
	if _, err := rand.Read(raw); err != nil {
		return OwnershipChallenge{}, err
	}
	value := "meil-verify=" + base64.RawURLEncoding.EncodeToString(raw)
	return OwnershipChallenge{
		Domain: domain,
		Name:   OwnershipTXTName + "." + domain,
		Value:  value,
	}, nil
}

func VerifyOwnership(expected string, txtRecords []string) bool {
	expected = strings.TrimSpace(expected)
	if expected == "" {
		return false
	}
	for _, record := range txtRecords {
		value := strings.TrimSpace(record)
		if subtle.ConstantTimeCompare([]byte(value), []byte(expected)) == 1 {
			return true
		}
	}
	return false
}
