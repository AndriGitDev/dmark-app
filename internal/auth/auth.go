// Package auth implements passwordless login tokens.
package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"time"
)

var (
	ErrInvalidToken  = errors.New("invalid login token")
	ErrExpiredToken  = errors.New("expired login token")
	ErrConsumedToken = errors.New("consumed login token")
)

type User struct {
	ID    string
	Email string
}

type LoginToken struct {
	UserID string
	Email  string
}

type Store interface {
	UpsertUserByEmail(ctx context.Context, email string) (User, error)
	CreateLoginToken(ctx context.Context, userID string, tokenHash []byte, expiresAt time.Time) error
	ConsumeLoginToken(ctx context.Context, tokenHash []byte) (LoginToken, error)
}

type Service struct {
	store Store
	ttl   time.Duration
}

func NewService(store Store, ttl time.Duration) *Service {
	if ttl <= 0 {
		ttl = 15 * time.Minute
	}
	return &Service{store: store, ttl: ttl}
}

func NormalizeEmail(input string) (string, error) {
	email := strings.TrimSpace(strings.ToLower(input))
	addr, err := mail.ParseAddress(email)
	if err != nil || addr.Address != email || !strings.Contains(email, "@") {
		return "", fmt.Errorf("invalid email address")
	}
	return email, nil
}

func (s *Service) StartLogin(ctx context.Context, input string) (string, string, error) {
	email, err := NormalizeEmail(input)
	if err != nil {
		return "", "", err
	}
	user, err := s.store.UpsertUserByEmail(ctx, email)
	if err != nil {
		return "", "", err
	}
	token, hash, err := newToken()
	if err != nil {
		return "", "", err
	}
	if err := s.store.CreateLoginToken(ctx, user.ID, hash, time.Now().Add(s.ttl)); err != nil {
		return "", "", err
	}
	return email, token, nil
}

func (s *Service) FinishLogin(ctx context.Context, token string) (LoginToken, error) {
	hash, err := hashToken(token)
	if err != nil {
		return LoginToken{}, ErrInvalidToken
	}
	login, err := s.store.ConsumeLoginToken(ctx, hash)
	if err != nil {
		if errors.Is(err, ErrExpiredToken) || errors.Is(err, ErrConsumedToken) {
			return LoginToken{}, err
		}
		return LoginToken{}, ErrInvalidToken
	}
	return login, nil
}

func newToken() (string, []byte, error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", nil, err
	}
	token := base64.RawURLEncoding.EncodeToString(raw)
	hash := sha256.Sum256([]byte(token))
	return token, hash[:], nil
}

func hashToken(token string) ([]byte, error) {
	if strings.TrimSpace(token) == "" {
		return nil, ErrInvalidToken
	}
	hash := sha256.Sum256([]byte(token))
	return hash[:], nil
}
