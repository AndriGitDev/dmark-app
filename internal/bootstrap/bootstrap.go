package bootstrap

import (
	"context"
	"fmt"
	"net/mail"
	"strings"

	"github.com/AndriGitDev/meil/internal/apikeys"
)

var DefaultScopes = []apikeys.Scope{
	apikeys.ScopeScanRead,
	apikeys.ScopeDomainRead,
	apikeys.ScopeDomainWrite,
	apikeys.ScopeReporterRead,
	apikeys.ScopeAPIKeyManage,
	apikeys.ScopeTeamManage,
}

type Config struct {
	OwnerEmail       string
	OrganizationName string
	APIKeyName       string
	APIKeyToken      string
	Scopes           []apikeys.Scope
}

type Result struct {
	Applied          bool
	UserID           string
	OrganizationID   string
	APIKeyCreated    bool
	APIKeyName       string
	ConfiguredScopes []apikeys.Scope
}

type Store interface {
	EnsureBootstrapOrganizationAPIKey(ctx context.Context, ownerEmail string, organizationName string, apiKeyName string, apiKeyToken string, scopes []apikeys.Scope) (Result, error)
}

func Run(ctx context.Context, store Store, cfg Config) (Result, error) {
	ownerEmail := strings.ToLower(strings.TrimSpace(cfg.OwnerEmail))
	organizationName := strings.TrimSpace(cfg.OrganizationName)
	apiKeyName := strings.TrimSpace(cfg.APIKeyName)
	apiKeyToken := strings.TrimSpace(cfg.APIKeyToken)

	if ownerEmail == "" && organizationName == "" && apiKeyToken == "" && apiKeyName == "" {
		return Result{}, nil
	}
	if store == nil {
		return Result{}, fmt.Errorf("bootstrap store is required")
	}
	if ownerEmail == "" {
		return Result{}, fmt.Errorf("BOOTSTRAP_OWNER_EMAIL is required when bootstrap is configured")
	}
	if _, err := mail.ParseAddress(ownerEmail); err != nil {
		return Result{}, fmt.Errorf("BOOTSTRAP_OWNER_EMAIL is invalid: %w", err)
	}
	if organizationName == "" {
		return Result{}, fmt.Errorf("BOOTSTRAP_ORGANIZATION_NAME is required when bootstrap is configured")
	}
	if apiKeyToken == "" {
		return Result{}, fmt.Errorf("BOOTSTRAP_API_KEY_TOKEN is required when bootstrap is configured")
	}
	if _, err := apikeys.Hash(apiKeyToken); err != nil {
		return Result{}, fmt.Errorf("BOOTSTRAP_API_KEY_TOKEN is invalid: %w", err)
	}
	if apiKeyName == "" {
		apiKeyName = "frontend-proxy"
	}
	scopes := normalizeScopes(cfg.Scopes)
	if len(scopes) == 0 {
		scopes = append([]apikeys.Scope(nil), DefaultScopes...)
	}
	return store.EnsureBootstrapOrganizationAPIKey(ctx, ownerEmail, organizationName, apiKeyName, apiKeyToken, scopes)
}

func normalizeScopes(scopes []apikeys.Scope) []apikeys.Scope {
	seen := map[apikeys.Scope]struct{}{}
	out := []apikeys.Scope{}
	for _, scope := range scopes {
		scope = apikeys.Scope(strings.TrimSpace(string(scope)))
		if scope == "" {
			continue
		}
		if _, ok := seen[scope]; ok {
			continue
		}
		seen[scope] = struct{}{}
		out = append(out, scope)
	}
	return out
}
