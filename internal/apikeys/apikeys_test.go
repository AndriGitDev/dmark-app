package apikeys

import (
	"errors"
	"strings"
	"testing"
)

func TestNewAndVerify(t *testing.T) {
	t.Parallel()

	key, hash, err := New()
	if err != nil {
		t.Fatalf("New returned error: %v", err)
	}
	if !strings.HasPrefix(key, Prefix) {
		t.Fatalf("key %q does not have prefix %q", key, Prefix)
	}
	if !Verify(key, hash) {
		t.Fatalf("Verify returned false for generated key")
	}
	if Verify(key+"x", hash) {
		t.Fatalf("Verify returned true for wrong key")
	}
}

func TestHashRejectsInvalidKey(t *testing.T) {
	t.Parallel()

	if _, err := Hash("not-a-meil-key"); !errors.Is(err, ErrInvalidKey) {
		t.Fatalf("Hash error = %v, want ErrInvalidKey", err)
	}
}

func TestAllows(t *testing.T) {
	t.Parallel()

	principal := Principal{Scopes: []Scope{ScopeScanRead, ScopeReporterRead}}
	if !Allows(principal, ScopeReporterRead) {
		t.Fatalf("Allows returned false for granted scope")
	}
	if Allows(principal, ScopeDomainWrite) {
		t.Fatalf("Allows returned true for missing scope")
	}
	principal.Revoked = true
	if Allows(principal, ScopeScanRead) {
		t.Fatalf("Allows returned true for revoked principal")
	}
}

func TestBearerToken(t *testing.T) {
	t.Parallel()

	token, err := BearerToken("Bearer meil_example")
	if err != nil {
		t.Fatalf("BearerToken returned error: %v", err)
	}
	if token != "meil_example" {
		t.Fatalf("token = %q, want meil_example", token)
	}
	if _, err := BearerToken("Basic abc"); !errors.Is(err, ErrInvalidKey) {
		t.Fatalf("BearerToken error = %v, want ErrInvalidKey", err)
	}
}

func TestAuthorizeBearer(t *testing.T) {
	t.Parallel()

	principal := Principal{Scopes: []Scope{ScopeReporterRead}}
	if err := AuthorizeBearer("Bearer meil_example", principal, ScopeReporterRead); err != nil {
		t.Fatalf("AuthorizeBearer returned error: %v", err)
	}
	if err := AuthorizeBearer("Bearer meil_example", principal, ScopeDomainWrite); !errors.Is(err, ErrInvalidKey) {
		t.Fatalf("AuthorizeBearer error = %v, want ErrInvalidKey", err)
	}
}
