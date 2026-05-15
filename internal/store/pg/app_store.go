package pg

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/AndriGitDev/meil/internal/auth"
	"github.com/AndriGitDev/meil/internal/domains"
	"github.com/AndriGitDev/meil/internal/scanner"
)

type AppStore struct {
	queries *Queries
}

func NewAppStore(queries *Queries) *AppStore {
	return &AppStore{queries: queries}
}

func (s *AppStore) UpsertUserByEmail(ctx context.Context, email string) (auth.User, error) {
	user, err := s.queries.UpsertUserByEmail(ctx, email)
	if err != nil {
		return auth.User{}, err
	}
	return auth.User{ID: uuidToString(user.ID), Email: user.Email}, nil
}

func (s *AppStore) CreateLoginToken(ctx context.Context, userID string, tokenHash []byte, expiresAt time.Time) error {
	userUUID, err := uuidFromString(userID)
	if err != nil {
		return err
	}
	_, err = s.queries.CreateLoginToken(ctx, CreateLoginTokenParams{
		UserID:    userUUID,
		TokenHash: tokenHash,
		ExpiresAt: pgtype.Timestamptz{Time: expiresAt, Valid: true},
	})
	return err
}

func (s *AppStore) ConsumeLoginToken(ctx context.Context, tokenHash []byte) (auth.LoginToken, error) {
	token, err := s.queries.ConsumeLoginToken(ctx, tokenHash)
	if err != nil {
		return auth.LoginToken{}, err
	}
	user, err := s.queries.GetUserByID(ctx, token.UserID)
	if err != nil {
		return auth.LoginToken{}, err
	}
	return auth.LoginToken{UserID: uuidToString(token.UserID), Email: user.Email}, nil
}

func (s *AppStore) CreateDomain(ctx context.Context, userID string, domain string) (domains.Domain, error) {
	userUUID, err := uuidFromString(userID)
	if err != nil {
		return domains.Domain{}, err
	}
	saved, err := s.queries.CreateDomain(ctx, CreateDomainParams{UserID: userUUID, Domain: domain})
	if err != nil {
		return domains.Domain{}, err
	}
	return domainFromRow(saved), nil
}

func (s *AppStore) ListDomains(ctx context.Context, userID string) ([]domains.Domain, error) {
	userUUID, err := uuidFromString(userID)
	if err != nil {
		return nil, err
	}
	rows, err := s.queries.ListDomainsWithLatestCheck(ctx, userUUID)
	if err != nil {
		return nil, err
	}
	result := make([]domains.Domain, 0, len(rows))
	for _, row := range rows {
		saved := domains.Domain{
			ID:        uuidToString(row.ID),
			Domain:    row.Domain,
			CreatedAt: row.CreatedAt.Time,
		}
		if row.CheckID.Valid {
			saved.LatestCheck = &domains.Check{
				SPF:       scanner.CheckResult{Name: "SPF", Status: scanner.Status(row.SpfStatus), Message: row.SpfMessage, Record: row.SpfRecord},
				DKIM:      scanner.CheckResult{Name: "DKIM", Status: scanner.Status(row.DkimStatus), Message: row.DkimMessage, Record: row.DkimRecord},
				DMARC:     scanner.CheckResult{Name: "DMARC", Status: scanner.Status(row.DmarcStatus), Message: row.DmarcMessage, Record: row.DmarcRecord},
				BIMI:      scanner.CheckResult{Name: "BIMI", Status: scanner.Status(row.BimiStatus), Message: row.BimiMessage, Record: row.BimiRecord},
				CheckedAt: row.CheckedAt.Time,
			}
		}
		result = append(result, saved)
	}
	return result, nil
}

func (s *AppStore) GetDomain(ctx context.Context, userID string, domainID string) (domains.Domain, error) {
	userUUID, err := uuidFromString(userID)
	if err != nil {
		return domains.Domain{}, err
	}
	domainUUID, err := uuidFromString(domainID)
	if err != nil {
		return domains.Domain{}, err
	}
	saved, err := s.queries.GetDomainForUser(ctx, GetDomainForUserParams{ID: domainUUID, UserID: userUUID})
	if err != nil {
		return domains.Domain{}, err
	}
	return domainFromRow(saved), nil
}

func (s *AppStore) DeleteDomain(ctx context.Context, userID string, domainID string) error {
	userUUID, err := uuidFromString(userID)
	if err != nil {
		return err
	}
	domainUUID, err := uuidFromString(domainID)
	if err != nil {
		return err
	}
	return s.queries.DeleteDomainForUser(ctx, DeleteDomainForUserParams{ID: domainUUID, UserID: userUUID})
}

func (s *AppStore) CreateCheck(ctx context.Context, domainID string, result scanner.Result) error {
	domainUUID, err := uuidFromString(domainID)
	if err != nil {
		return err
	}
	_, err = s.queries.CreateDomainCheck(ctx, CreateDomainCheckParams{
		DomainID:     domainUUID,
		SpfStatus:    string(result.SPF.Status),
		SpfMessage:   result.SPF.Message,
		SpfRecord:    result.SPF.Record,
		DkimStatus:   string(result.DKIM.Status),
		DkimMessage:  result.DKIM.Message,
		DkimRecord:   result.DKIM.Record,
		DmarcStatus:  string(result.DMARC.Status),
		DmarcMessage: result.DMARC.Message,
		DmarcRecord:  result.DMARC.Record,
		BimiStatus:   string(result.BIMI.Status),
		BimiMessage:  result.BIMI.Message,
		BimiRecord:   result.BIMI.Record,
	})
	return err
}

func domainFromRow(row Domain) domains.Domain {
	return domains.Domain{ID: uuidToString(row.ID), Domain: row.Domain, CreatedAt: row.CreatedAt.Time}
}

func uuidFromString(value string) (pgtype.UUID, error) {
	var id pgtype.UUID
	if err := id.Scan(value); err != nil {
		return pgtype.UUID{}, fmt.Errorf("parsing uuid %q: %w", value, err)
	}
	return id, nil
}

func uuidToString(value pgtype.UUID) string {
	return fmt.Sprintf("%x-%x-%x-%x-%x", value.Bytes[0:4], value.Bytes[4:6], value.Bytes[6:8], value.Bytes[8:10], value.Bytes[10:16])
}
