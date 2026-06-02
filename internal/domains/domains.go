// Package domains stores user-owned domain checks.
package domains

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/AndriGitDev/meil/internal/scanner"
)

type Domain struct {
	ID          string
	Domain      string
	CreatedAt   time.Time
	LatestCheck *Check
}

type Check struct {
	SPF              scanner.CheckResult
	DKIM             scanner.CheckResult
	DMARC            scanner.CheckResult
	BIMI             scanner.CheckResult
	CheckedAt        time.Time
	EvidenceSnapshot EvidenceSnapshot
}

type Store interface {
	CreateDomain(ctx context.Context, userID string, domain string) (Domain, error)
	ListDomains(ctx context.Context, userID string) ([]Domain, error)
	GetDomain(ctx context.Context, userID string, domainID string) (Domain, error)
	DeleteDomain(ctx context.Context, userID string, domainID string) error
	CreateCheck(ctx context.Context, domainID string, result scanner.Result) error
}

type Scanner interface {
	Scan(ctx context.Context, domain string) (scanner.Result, error)
}

type TXTResolver interface {
	LookupTXT(ctx context.Context, name string) ([]string, error)
}

type Service struct {
	store    Store
	scanner  Scanner
	resolver TXTResolver
}

type ImportResult struct {
	Created int
	Domains []string
}

func NewService(store Store, emailScanner Scanner) *Service {
	return &Service{store: store, scanner: emailScanner}
}

func (s *Service) WithResolver(resolver TXTResolver) *Service {
	s.resolver = resolver
	return s
}

func (s *Service) AddDomain(ctx context.Context, userID string, input string) error {
	domain, err := scanner.NormalizeDomain(input)
	if err != nil {
		return err
	}
	saved, err := s.store.CreateDomain(ctx, userID, domain)
	if err != nil {
		return err
	}
	return s.ScanDomain(ctx, userID, saved.ID)
}

func (s *Service) ScanDomain(ctx context.Context, userID string, domainID string) error {
	saved, err := s.store.GetDomain(ctx, userID, domainID)
	if err != nil {
		return err
	}
	result, err := s.scanner.Scan(ctx, saved.Domain)
	if err != nil {
		return err
	}
	return s.store.CreateCheck(ctx, saved.ID, result)
}

func (s *Service) ListDomains(ctx context.Context, userID string) ([]Domain, error) {
	return s.store.ListDomains(ctx, userID)
}

func (s *Service) GetDomain(ctx context.Context, userID string, domainID string) (Domain, error) {
	return s.store.GetDomain(ctx, userID, domainID)
}

func (s *Service) DeleteDomain(ctx context.Context, userID string, domainID string) error {
	return s.store.DeleteDomain(ctx, userID, domainID)
}

func (s *Service) ImportCSV(ctx context.Context, userID string, reader io.Reader) (ImportResult, error) {
	domainNames, err := ParseCSV(reader)
	if err != nil {
		return ImportResult{}, err
	}
	result := ImportResult{Domains: make([]string, 0, len(domainNames))}
	for _, domainName := range domainNames {
		if err := s.AddDomain(ctx, userID, domainName); err != nil {
			return result, fmt.Errorf("importing %s: %w", domainName, err)
		}
		result.Created++
		result.Domains = append(result.Domains, domainName)
	}
	return result, nil
}

func (s *Service) ExportCSV(ctx context.Context, userID string, writer io.Writer) error {
	saved, err := s.ListDomains(ctx, userID)
	if err != nil {
		return err
	}
	return WriteCSV(writer, saved)
}

func (s *Service) OwnershipChallenge(ctx context.Context, userID string, domainID string) (OwnershipChallenge, error) {
	saved, err := s.store.GetDomain(ctx, userID, domainID)
	if err != nil {
		return OwnershipChallenge{}, err
	}
	return NewOwnershipChallenge(saved.Domain)
}

func (s *Service) VerifyOwnership(ctx context.Context, userID string, domainID string, expected string) (bool, error) {
	if s.resolver == nil {
		return false, fmt.Errorf("ownership resolver is not configured")
	}
	saved, err := s.store.GetDomain(ctx, userID, domainID)
	if err != nil {
		return false, err
	}
	records, err := s.resolver.LookupTXT(ctx, OwnershipTXTName+"."+saved.Domain)
	if err != nil {
		return false, err
	}
	return VerifyOwnership(expected, records), nil
}
