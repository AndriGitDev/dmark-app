package domains

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/AndriGitDev/meil/internal/scanner"
)

type fakeStore struct {
	domains []Domain
	check   scanner.Result
}

func (s *fakeStore) CreateDomain(ctx context.Context, userID string, domain string) (Domain, error) {
	saved := Domain{ID: "domain-" + domain, Domain: domain}
	s.domains = append(s.domains, saved)
	return saved, nil
}

func (s *fakeStore) ListDomains(ctx context.Context, userID string) ([]Domain, error) {
	return append([]Domain(nil), s.domains...), nil
}

func (s *fakeStore) GetDomain(ctx context.Context, userID string, domainID string) (Domain, error) {
	for _, domain := range s.domains {
		if domain.ID == domainID {
			return domain, nil
		}
	}
	return Domain{}, nil
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

type fakeTXTResolver struct {
	records []string
}

func (r fakeTXTResolver) LookupTXT(ctx context.Context, name string) ([]string, error) {
	return r.records, nil
}

func TestAddDomainNormalizesAndPersistsScan(t *testing.T) {
	t.Parallel()

	store := &fakeStore{}
	service := NewService(store, fakeScanner{})
	if err := service.AddDomain(context.Background(), "user-1", " HTTPS://Example.COM/path "); err != nil {
		t.Fatalf("AddDomain returned error: %v", err)
	}
	if store.domains[0].Domain != "example.com" {
		t.Fatalf("domain = %q, want example.com", store.domains[0].Domain)
	}
	if store.check.Domain != "example.com" || store.check.SPF.Status != scanner.StatusPass {
		t.Fatalf("unexpected persisted check: %#v", store.check)
	}
}

func TestImportCSVAddsEachDomain(t *testing.T) {
	t.Parallel()

	store := &fakeStore{}
	service := NewService(store, fakeScanner{})
	result, err := service.ImportCSV(context.Background(), "user-1", strings.NewReader("domain\nexample.com\nmail.example.org\n"))
	if err != nil {
		t.Fatalf("ImportCSV returned error: %v", err)
	}
	if result.Created != 2 || len(store.domains) != 2 {
		t.Fatalf("result = %+v domains = %+v, want two imported domains", result, store.domains)
	}
}

func TestExportCSVWritesSavedDomains(t *testing.T) {
	t.Parallel()

	store := &fakeStore{domains: []Domain{{ID: "domain-1", Domain: "example.com"}}}
	service := NewService(store, fakeScanner{})
	var out bytes.Buffer
	if err := service.ExportCSV(context.Background(), "user-1", &out); err != nil {
		t.Fatalf("ExportCSV returned error: %v", err)
	}
	if !strings.Contains(out.String(), "example.com") {
		t.Fatalf("export missing domain: %s", out.String())
	}
}

func TestOwnershipChallengeUsesSavedDomain(t *testing.T) {
	t.Parallel()

	store := &fakeStore{domains: []Domain{{ID: "domain-1", Domain: "example.com"}}}
	service := NewService(store, fakeScanner{})
	challenge, err := service.OwnershipChallenge(context.Background(), "user-1", "domain-1")
	if err != nil {
		t.Fatalf("OwnershipChallenge returned error: %v", err)
	}
	if challenge.Name != "_meil-verify.example.com" || !strings.HasPrefix(challenge.Value, "meil-verify=") {
		t.Fatalf("unexpected challenge: %+v", challenge)
	}
}

func TestVerifyOwnershipUsesResolver(t *testing.T) {
	t.Parallel()

	store := &fakeStore{domains: []Domain{{ID: "domain-1", Domain: "example.com"}}}
	service := NewService(store, fakeScanner{}).WithResolver(fakeTXTResolver{records: []string{"meil-verify=abc"}})
	ok, err := service.VerifyOwnership(context.Background(), "user-1", "domain-1", "meil-verify=abc")
	if err != nil {
		t.Fatalf("VerifyOwnership returned error: %v", err)
	}
	if !ok {
		t.Fatalf("VerifyOwnership returned false, want true")
	}
}
