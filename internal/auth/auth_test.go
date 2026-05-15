package auth

import (
	"context"
	"errors"
	"testing"
	"time"
)

type memoryStore struct {
	users  map[string]User
	tokens map[string]LoginToken
}

func newMemoryStore() *memoryStore {
	return &memoryStore{users: map[string]User{}, tokens: map[string]LoginToken{}}
}

func (s *memoryStore) UpsertUserByEmail(ctx context.Context, email string) (User, error) {
	if user, ok := s.users[email]; ok {
		return user, nil
	}
	user := User{ID: "user-1", Email: email}
	s.users[email] = user
	return user, nil
}

func (s *memoryStore) CreateLoginToken(ctx context.Context, userID string, tokenHash []byte, expiresAt time.Time) error {
	s.tokens[string(tokenHash)] = LoginToken{UserID: userID, Email: "person@example.com"}
	return nil
}

func (s *memoryStore) ConsumeLoginToken(ctx context.Context, tokenHash []byte) (LoginToken, error) {
	login, ok := s.tokens[string(tokenHash)]
	if !ok {
		return LoginToken{}, errors.New("not found")
	}
	delete(s.tokens, string(tokenHash))
	return login, nil
}

func TestStartAndFinishLogin(t *testing.T) {
	t.Parallel()

	service := NewService(newMemoryStore(), time.Minute)
	email, token, err := service.StartLogin(context.Background(), " Person@Example.COM ")
	if err != nil {
		t.Fatalf("StartLogin returned error: %v", err)
	}
	if email != "person@example.com" {
		t.Fatalf("email = %q, want %q", email, "person@example.com")
	}
	if token == "" {
		t.Fatal("token is empty")
	}

	login, err := service.FinishLogin(context.Background(), token)
	if err != nil {
		t.Fatalf("FinishLogin returned error: %v", err)
	}
	if login.UserID != "user-1" {
		t.Fatalf("user ID = %q, want user-1", login.UserID)
	}
}

func TestFinishLoginRejectsConsumedToken(t *testing.T) {
	t.Parallel()

	service := NewService(newMemoryStore(), time.Minute)
	_, token, err := service.StartLogin(context.Background(), "person@example.com")
	if err != nil {
		t.Fatalf("StartLogin returned error: %v", err)
	}
	if _, err := service.FinishLogin(context.Background(), token); err != nil {
		t.Fatalf("first FinishLogin returned error: %v", err)
	}
	if _, err := service.FinishLogin(context.Background(), token); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("second FinishLogin error = %v, want ErrInvalidToken", err)
	}
}

func TestNormalizeEmailRejectsInvalidEmail(t *testing.T) {
	t.Parallel()

	if _, err := NormalizeEmail("not-email"); err == nil {
		t.Fatal("NormalizeEmail returned nil error")
	}
}
