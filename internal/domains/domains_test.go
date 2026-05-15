package domains

import (
	"context"
	"testing"

	"github.com/AndriGitDev/meil/internal/scanner"
)

type fakeStore struct {
	domain Domain
	check  scanner.Result
}

func (s *fakeStore) CreateDomain(ctx context.Context, userID string, domain string) (Domain, error) {
	s.domain = Domain{ID: "domain-1", Domain: domain}
	return s.domain, nil
}

func (s *fakeStore) ListDomains(ctx context.Context, userID string) ([]Domain, error) {
	return []Domain{s.domain}, nil
}

func (s *fakeStore) GetDomain(ctx context.Context, userID string, domainID string) (Domain, error) {
	return s.domain, nil
}

func (s *fakeStore) DeleteDomain(ctx context.Context, userID string, domainID string) error {
	return nil
}

func (s *fakeStore) CreateCheck(ctx context.Context, domainID string, result scanner.Result) error {
	s.check = result
	return nil
}

type fakeScanner struct{}

func (fakeScanner) Scan(ctx context.Context, domain string) (scanner.Result, error) {
	return scanner.Result{Domain: domain, SPF: scanner.CheckResult{Name: "SPF", Status: scanner.StatusPass}}, nil
}

func TestAddDomainNormalizesAndPersistsScan(t *testing.T) {
	t.Parallel()

	store := &fakeStore{}
	service := NewService(store, fakeScanner{})
	if err := service.AddDomain(context.Background(), "user-1", " HTTPS://Example.COM/path "); err != nil {
		t.Fatalf("AddDomain returned error: %v", err)
	}
	if store.domain.Domain != "example.com" {
		t.Fatalf("domain = %q, want example.com", store.domain.Domain)
	}
	if store.check.Domain != "example.com" || store.check.SPF.Status != scanner.StatusPass {
		t.Fatalf("unexpected persisted check: %#v", store.check)
	}
}
