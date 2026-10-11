package pg

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/AndriGitDev/meil/internal/workspace"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
)

func TestPostgresWorkspaceIsolationAndConcurrentSave(t *testing.T) {
	if os.Getenv("MEIL_DB_INTEGRATION") != "1" {
		t.Skip("set MEIL_DB_INTEGRATION=1 with DATABASE_URL")
	}
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Fatal("DATABASE_URL required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	if err := goose.SetDialect("postgres"); err != nil {
		t.Fatal(err)
	}
	if err := goose.UpContext(ctx, db, "../../../migrations/postgres"); err != nil {
		t.Fatal(err)
	}
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	store := workspace.NewPostgresStore(pool)
	ids := []string{}
	for i := 0; i < 2; i++ {
		var id string
		if err := pool.QueryRow(ctx, `INSERT INTO users(email) VALUES($1) RETURNING id`, fmt.Sprintf("workspace-%d-%d@example.com", time.Now().UnixNano(), i)).Scan(&id); err != nil {
			t.Fatal(err)
		}
		ids = append(ids, id)
	}
	defer func() {
		for _, id := range ids {
			_, _ = pool.Exec(context.Background(), `DELETE FROM users WHERE id=$1`, id)
		}
	}()
	d, err := store.Load(ctx, ids[0])
	if err != nil || d.Version != 0 || len(d.Domains) != 0 {
		t.Fatal(d, err)
	}
	saved, err := store.Save(ctx, ids[0], 0, []workspace.Domain{{Domain: "example.com", Completed: []string{"spf"}, Selector: "s1"}})
	if err != nil || saved.Version != 1 {
		t.Fatal(saved, err)
	}
	other, err := store.Load(ctx, ids[1])
	if err != nil || len(other.Domains) != 0 {
		t.Fatal("user isolation", other, err)
	}
	if _, err := store.Save(ctx, ids[0], 0, []workspace.Domain{}); !errors.Is(err, workspace.ErrConflict) {
		t.Fatal("stale insert accepted", err)
	}
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		go func() {
			_, err := store.Save(ctx, ids[0], 1, []workspace.Domain{{Domain: "kastro.is", Completed: []string{}}})
			results <- err
		}()
	}
	success, conflict := 0, 0
	for i := 0; i < 2; i++ {
		err := <-results
		if err == nil {
			success++
		} else if errors.Is(err, workspace.ErrConflict) {
			conflict++
		} else {
			t.Fatal(err)
		}
	}
	if success != 1 || conflict != 1 {
		t.Fatal("concurrent saves", success, conflict)
	}
	d, err = store.Load(ctx, ids[0])
	if err != nil || d.Version != 2 || d.Domains[0].Domain != "kastro.is" {
		t.Fatal(d, err)
	}
	if _, err := pool.Exec(ctx, `DELETE FROM users WHERE id=$1`, ids[0]); err != nil {
		t.Fatal(err)
	}
	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM account_workspaces WHERE user_id=$1`, ids[0]).Scan(&count); err != nil || count != 0 {
		t.Fatal("cascade", count, err)
	}
}
