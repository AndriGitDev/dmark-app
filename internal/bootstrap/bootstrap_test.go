package bootstrap

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/AndriGitDev/meil/internal/apikeys"
)

type fakeStore struct {
	calls            int
	result           Result
	err              error
	ownerEmail       string
	organizationName string
	apiKeyName       string
	apiKeyToken      string
	scopes           []apikeys.Scope
}

func (s *fakeStore) EnsureBootstrapOrganizationAPIKey(ctx context.Context, ownerEmail string, organizationName string, apiKeyName string, apiKeyToken string, scopes []apikeys.Scope) (Result, error) {
	s.calls++
	s.ownerEmail = ownerEmail
	s.organizationName = organizationName
	s.apiKeyName = apiKeyName
	s.apiKeyToken = apiKeyToken
	s.scopes = append([]apikeys.Scope(nil), scopes...)
	return s.result, s.err
}

func TestRunSkipsWhenUnconfigured(t *testing.T) {
	store := &fakeStore{}
	result, err := Run(context.Background(), store, Config{})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if store.calls != 0 {
		t.Fatalf("store calls = %d, want 0", store.calls)
	}
	if result.Applied {
		t.Fatalf("result applied = true, want false")
	}
}

func TestRunValidatesCompleteConfiguration(t *testing.T) {
	validToken, _, err := apikeys.New()
	if err != nil {
		t.Fatalf("new API key: %v", err)
	}
	tests := []struct {
		name string
		cfg  Config
	}{
		{name: "missing owner", cfg: Config{OrganizationName: "DMARK", APIKeyToken: validToken}},
		{name: "invalid owner", cfg: Config{OwnerEmail: "not-email", OrganizationName: "DMARK", APIKeyToken: validToken}},
		{name: "missing organization", cfg: Config{OwnerEmail: "owner@example.com", APIKeyToken: validToken}},
		{name: "missing token", cfg: Config{OwnerEmail: "owner@example.com", OrganizationName: "DMARK"}},
		{name: "invalid token", cfg: Config{OwnerEmail: "owner@example.com", OrganizationName: "DMARK", APIKeyToken: "not-a-meil-key"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := Run(context.Background(), &fakeStore{}, tt.cfg); err == nil {
				t.Fatal("Run error = nil, want validation error")
			}
		})
	}
}

func TestRunNormalizesAndDelegates(t *testing.T) {
	token, _, err := apikeys.New()
	if err != nil {
		t.Fatalf("new API key: %v", err)
	}
	store := &fakeStore{result: Result{Applied: true, UserID: "user-1", OrganizationID: "org-1", APIKeyCreated: true}}
	result, err := Run(context.Background(), store, Config{
		OwnerEmail:       " Owner@Example.COM ",
		OrganizationName: " DMARK ",
		APIKeyToken:      " " + token + " ",
		Scopes:           []apikeys.Scope{apikeys.ScopeDomainRead, "", apikeys.ScopeDomainRead, apikeys.ScopeReporterRead},
	})
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if !result.Applied || !result.APIKeyCreated {
		t.Fatalf("result = %+v, want applied created", result)
	}
	if store.calls != 1 {
		t.Fatalf("store calls = %d, want 1", store.calls)
	}
	if store.ownerEmail != "owner@example.com" || store.organizationName != "DMARK" || store.apiKeyName != "frontend-proxy" || store.apiKeyToken != token {
		t.Fatalf("delegated values = email %q org %q name %q token matches %v", store.ownerEmail, store.organizationName, store.apiKeyName, store.apiKeyToken == token)
	}
	wantScopes := []apikeys.Scope{apikeys.ScopeDomainRead, apikeys.ScopeReporterRead}
	if !reflect.DeepEqual(store.scopes, wantScopes) {
		t.Fatalf("scopes = %+v, want %+v", store.scopes, wantScopes)
	}
}

func TestRunPropagatesStoreError(t *testing.T) {
	token, _, err := apikeys.New()
	if err != nil {
		t.Fatalf("new API key: %v", err)
	}
	boom := errors.New("boom")
	_, err = Run(context.Background(), &fakeStore{err: boom}, Config{OwnerEmail: "owner@example.com", OrganizationName: "DMARK", APIKeyToken: token})
	if !errors.Is(err, boom) {
		t.Fatalf("Run error = %v, want %v", err, boom)
	}
}
