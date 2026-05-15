// Package domains stores user-owned domain checks.
package domains

import (
	"context"
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
	SPF       scanner.CheckResult
	DKIM      scanner.CheckResult
	DMARC     scanner.CheckResult
	BIMI      scanner.CheckResult
	CheckedAt time.Time
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

type Service struct {
	store   Store
	scanner Scanner
}

func NewService(store Store, emailScanner Scanner) *Service {
	return &Service{store: store, scanner: emailScanner}
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

func (s *Service) DeleteDomain(ctx context.Context, userID string, domainID string) error {
	return s.store.DeleteDomain(ctx, userID, domainID)
}
