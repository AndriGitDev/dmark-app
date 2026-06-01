package apikeys

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

type recordingAuthenticator struct {
	principal Principal
	err       error
	token     string
	calls     int
}

func (a *recordingAuthenticator) AuthenticateAPIKey(ctx context.Context, token string) (Principal, error) {
	a.calls++
	a.token = token
	if a.err != nil {
		return Principal{}, a.err
	}
	return a.principal, nil
}

func TestOwnerAdminTokenAuthenticatorFailsClosedWhenTokenBlank(t *testing.T) {
	t.Parallel()

	fallback := &recordingAuthenticator{err: ErrInvalidKey}
	authenticator := NewOwnerAdminTokenAuthenticator("   ", fallback)

	principal, err := authenticator.AuthenticateAPIKey(context.Background(), "owner-token")
	if !errors.Is(err, ErrInvalidKey) {
		t.Fatalf("AuthenticateAPIKey error = %v, want ErrInvalidKey", err)
	}
	if principal.GlobalAdmin || Allows(principal, ScopeAdminManage) {
		t.Fatalf("blank owner token resolved admin principal: %+v", principal)
	}
	if fallback.calls != 1 || fallback.token != "owner-token" {
		t.Fatalf("fallback calls=%d token=%q, want one call with original token", fallback.calls, fallback.token)
	}
}

func TestOwnerAdminTokenAuthenticatorResolvesExactOwnerToken(t *testing.T) {
	t.Parallel()

	fallback := &recordingAuthenticator{err: ErrInvalidKey}
	authenticator := NewOwnerAdminTokenAuthenticator("owner-token", fallback)

	principal, err := authenticator.AuthenticateAPIKey(context.Background(), "owner-token")
	if err != nil {
		t.Fatalf("AuthenticateAPIKey returned error: %v", err)
	}
	if !principal.GlobalAdmin {
		t.Fatalf("GlobalAdmin = false, want true")
	}
	if !Allows(principal, ScopeAdminManage) {
		t.Fatalf("principal does not allow admin:manage: %+v", principal)
	}
	if principal.OrganizationID != "" {
		t.Fatalf("OrganizationID = %q, want empty owner-only admin principal", principal.OrganizationID)
	}
	if fallback.calls != 0 {
		t.Fatalf("fallback called %d times for owner token, want 0", fallback.calls)
	}
}

func TestOwnerAdminTokenAuthenticatorLeavesOrdinaryAPIKeyPathUnchanged(t *testing.T) {
	t.Parallel()

	ordinary := Principal{OrganizationID: "org-1", Scopes: []Scope{ScopeAdminManage}, GlobalAdmin: false}
	fallback := &recordingAuthenticator{principal: ordinary}
	authenticator := NewOwnerAdminTokenAuthenticator("owner-token", fallback)

	principal, err := authenticator.AuthenticateAPIKey(context.Background(), "meil_ordinary")
	if err != nil {
		t.Fatalf("AuthenticateAPIKey returned error: %v", err)
	}
	if !reflect.DeepEqual(principal, ordinary) {
		t.Fatalf("principal = %+v, want unchanged ordinary principal %+v", principal, ordinary)
	}
	if fallback.calls != 1 || fallback.token != "meil_ordinary" {
		t.Fatalf("fallback calls=%d token=%q, want one ordinary token call", fallback.calls, fallback.token)
	}
}

func TestOwnerAdminTokenAuthenticatorWrongTokenDoesNotResolveAdmin(t *testing.T) {
	t.Parallel()

	fallback := &recordingAuthenticator{err: ErrInvalidKey}
	authenticator := NewOwnerAdminTokenAuthenticator("owner-token", fallback)

	principal, err := authenticator.AuthenticateAPIKey(context.Background(), "wrong-token")
	if !errors.Is(err, ErrInvalidKey) {
		t.Fatalf("AuthenticateAPIKey error = %v, want ErrInvalidKey", err)
	}
	if principal.GlobalAdmin || Allows(principal, ScopeAdminManage) {
		t.Fatalf("wrong token resolved admin principal: %+v", principal)
	}
	if fallback.calls != 1 || fallback.token != "wrong-token" {
		t.Fatalf("fallback calls=%d token=%q, want one wrong token call", fallback.calls, fallback.token)
	}
}

func TestOwnerAdminTokenAuthenticatorNilFallbackRejectsNonOwnerToken(t *testing.T) {
	t.Parallel()

	authenticator := NewOwnerAdminTokenAuthenticator("owner-token", nil)

	if _, err := authenticator.AuthenticateAPIKey(context.Background(), "wrong-token"); !errors.Is(err, ErrInvalidKey) {
		t.Fatalf("AuthenticateAPIKey error = %v, want ErrInvalidKey", err)
	}
}
