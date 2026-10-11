// Package workspace stores the optional free account's domain checklist.
package workspace

import (
	"context"
	"encoding/json"
	"errors"
	"regexp"
	"strings"

	"github.com/AndriGitDev/meil/internal/scanner"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrConflict = errors.New("workspace changed")
	ErrInvalid  = errors.New("invalid workspace")
)

type Domain struct {
	Domain    string   `json:"domain"`
	Completed []string `json:"completed"`
	Selector  string   `json:"selector,omitempty"`
}
type Document struct {
	Domains []Domain `json:"domains"`
	Version int64    `json:"version"`
}
type Store interface {
	Load(context.Context, string) (Document, error)
	Save(context.Context, string, int64, []Domain) (Document, error)
}

var (
	steps         = []string{"senders", "spf", "dkim", "reports", "dmarc", "bimi"}
	selectorLabel = regexp.MustCompile(`^[A-Za-z0-9_](?:[A-Za-z0-9_-]{0,61}[A-Za-z0-9_])?$`)
)

func Validate(domains []Domain) ([]Domain, error) {
	if len(domains) > 20 {
		return nil, ErrInvalid
	}
	result := make([]Domain, 0, len(domains))
	seen := map[string]bool{}
	for _, item := range domains {
		name, err := scanner.NormalizeDomain(item.Domain)
		if err != nil || len(name) > 253 || seen[name] {
			return nil, ErrInvalid
		}
		seen[name] = true
		selector := strings.TrimSpace(item.Selector)
		if len(selector) > 128 {
			return nil, ErrInvalid
		}
		if selector != "" {
			for _, label := range strings.Split(selector, ".") {
				if !selectorLabel.MatchString(label) {
					return nil, ErrInvalid
				}
			}
		}
		if len(item.Completed) > 6 {
			return nil, ErrInvalid
		}
		selected := map[string]bool{}
		for _, step := range item.Completed {
			valid := false
			for _, known := range steps {
				if step == known {
					valid = true
				}
			}
			if !valid {
				return nil, ErrInvalid
			}
			selected[step] = true
		}
		completed := []string{}
		for _, step := range steps {
			if selected[step] {
				completed = append(completed, step)
			}
		}
		result = append(result, Domain{Domain: name, Completed: completed, Selector: selector})
	}
	return result, nil
}

type PostgresStore struct{ pool *pgxpool.Pool }

func NewPostgresStore(pool *pgxpool.Pool) *PostgresStore { return &PostgresStore{pool: pool} }
func (s *PostgresStore) Load(ctx context.Context, userID string) (Document, error) {
	d := Document{Domains: []Domain{}}
	var raw []byte
	err := s.pool.QueryRow(ctx, `SELECT domains,version FROM account_workspaces WHERE user_id=$1`, userID).Scan(&raw, &d.Version)
	if errors.Is(err, pgx.ErrNoRows) {
		return d, nil
	}
	if err != nil {
		return d, err
	}
	err = json.Unmarshal(raw, &d.Domains)
	return d, err
}

func (s *PostgresStore) Save(ctx context.Context, userID string, version int64, domains []Domain) (Document, error) {
	var d Document
	if version < 0 {
		return d, ErrInvalid
	}
	values, err := Validate(domains)
	if err != nil {
		return d, err
	}
	raw, err := json.Marshal(values)
	if err != nil {
		return d, err
	}
	var saved []byte
	if version == 0 {
		err = s.pool.QueryRow(ctx, `INSERT INTO account_workspaces(user_id,domains,version) VALUES($1,$2,1) ON CONFLICT(user_id) DO NOTHING RETURNING domains,version`, userID, raw).Scan(&saved, &d.Version)
	} else {
		err = s.pool.QueryRow(ctx, `UPDATE account_workspaces SET domains=$2,version=version+1,updated_at=now() WHERE user_id=$1 AND version=$3 RETURNING domains,version`, userID, raw, version).Scan(&saved, &d.Version)
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return d, ErrConflict
	}
	if err != nil {
		return d, err
	}
	err = json.Unmarshal(saved, &d.Domains)
	return d, err
}
