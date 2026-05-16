package apikeys

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"strings"
	"time"
)

const Prefix = "meil_"

var ErrInvalidKey = errors.New("invalid API key")

type Scope string

const (
	ScopeScanRead      Scope = "scan:read"
	ScopeDomainRead    Scope = "domain:read"
	ScopeDomainWrite   Scope = "domain:write"
	ScopeReporterRead  Scope = "reporter:read"
	ScopeBillingManage Scope = "billing:manage"
	ScopePrivacyManage Scope = "privacy:manage"
	ScopeAPIKeyManage  Scope = "api_key:manage"
	ScopeTeamManage    Scope = "team:manage"
	ScopeAdminManage   Scope = "admin:manage"
	ScopeWebhookManage Scope = "webhook:manage"
)

type Principal struct {
	OrganizationID string
	Scopes         []Scope
	Revoked        bool
}

type Key struct {
	ID             string
	OrganizationID string
	Name           string
	Scopes         []Scope
	Token          string
	LastUsedAt     time.Time
	RevokedAt      time.Time
	CreatedAt      time.Time
}

func New() (string, []byte, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", nil, err
	}
	key := Prefix + base64.RawURLEncoding.EncodeToString(raw)
	hash, err := Hash(key)
	if err != nil {
		return "", nil, err
	}
	return key, hash, nil
}

func Hash(key string) ([]byte, error) {
	key = strings.TrimSpace(key)
	if !strings.HasPrefix(key, Prefix) {
		return nil, ErrInvalidKey
	}
	sum := sha256.Sum256([]byte(key))
	return sum[:], nil
}

func Verify(key string, hash []byte) bool {
	keyHash, err := Hash(key)
	if err != nil {
		return false
	}
	return subtle.ConstantTimeCompare(keyHash, hash) == 1
}

func Allows(principal Principal, scope Scope) bool {
	if principal.Revoked {
		return false
	}
	for _, allowed := range principal.Scopes {
		if allowed == scope {
			return true
		}
	}
	return false
}

func BearerToken(header string) (string, error) {
	header = strings.TrimSpace(header)
	prefix, token, ok := strings.Cut(header, " ")
	if !ok || !strings.EqualFold(prefix, "Bearer") || strings.TrimSpace(token) == "" {
		return "", ErrInvalidKey
	}
	return strings.TrimSpace(token), nil
}

func AuthorizeBearer(header string, principal Principal, required Scope) error {
	if _, err := BearerToken(header); err != nil {
		return err
	}
	if !Allows(principal, required) {
		return ErrInvalidKey
	}
	return nil
}
