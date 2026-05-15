package main

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/alexedwards/scs/v2"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/spf13/cobra"

	"github.com/AndriGitDev/meil/internal/auth"
	"github.com/AndriGitDev/meil/internal/config"
	"github.com/AndriGitDev/meil/internal/domains"
	"github.com/AndriGitDev/meil/internal/notifier"
	"github.com/AndriGitDev/meil/internal/scanner"
	"github.com/AndriGitDev/meil/internal/store/pg"
	"github.com/AndriGitDev/meil/internal/web"
)

func main() {
	if err := newRootCommand().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func newRootCommand() *cobra.Command {
	root := &cobra.Command{
		Use:   "meil",
		Short: "Email-auth monitoring for domain owners",
	}

	root.AddCommand(newServeCommand())
	root.AddCommand(newHealthcheckCommand())
	root.AddCommand(stubCommand("worker"))
	root.AddCommand(stubCommand("ingest"))
	root.AddCommand(newMigrateCommand())
	root.AddCommand(newScanCommand())

	return root
}

func newMigrateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "migrate [up|down|status]",
		Short: "Run database migrations",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}
			db, err := sql.Open("pgx", cfg.DatabaseURL)
			if err != nil {
				return fmt.Errorf("opening database: %w", err)
			}
			defer func() { _ = db.Close() }()
			if err := goose.SetDialect("postgres"); err != nil {
				return fmt.Errorf("setting migration dialect: %w", err)
			}

			switch args[0] {
			case "up":
				return goose.Up(db, "migrations/postgres")
			case "down":
				return goose.Down(db, "migrations/postgres")
			case "status":
				return goose.Status(db, "migrations/postgres")
			default:
				return fmt.Errorf("unknown migration direction %q", args[0])
			}
		},
	}
	return cmd
}

func newScanCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "scan <domain>",
		Short: "Run a one-off email-auth scan",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := scanner.New(nil).Scan(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			return printScanResult(cmd.OutOrStdout(), result)
		},
	}
}

func printScanResult(w io.Writer, result scanner.Result) error {
	if _, err := fmt.Fprintf(w, "Domain: %s\n", result.Domain); err != nil {
		return err
	}
	for _, check := range []scanner.CheckResult{result.SPF, result.DMARC, result.BIMI, result.DKIM} {
		if _, err := fmt.Fprintf(w, "%s: %s - %s\n", check.Name, check.Status, check.Message); err != nil {
			return err
		}
		if check.Record != "" {
			if _, err := fmt.Fprintf(w, "  %s\n", check.Record); err != nil {
				return err
			}
		}
	}
	return nil
}

func newHealthcheckCommand() *cobra.Command {
	return &cobra.Command{
		Use:    "healthcheck",
		Short:  "Check the HTTP server health endpoint",
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			client := http.Client{Timeout: 2 * time.Second}
			resp, err := client.Get("http://127.0.0.1:8080/healthz")
			if err != nil {
				return fmt.Errorf("checking health endpoint: %w", err)
			}
			defer func() {
				_ = resp.Body.Close()
			}()

			if resp.StatusCode != http.StatusOK {
				return fmt.Errorf("checking health endpoint: status %d", resp.StatusCode)
			}
			return nil
		},
	}
}

func stubCommand(name string) *cobra.Command {
	return &cobra.Command{
		Use:   name,
		Short: fmt.Sprintf("Run %s role", name),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, err := fmt.Fprintln(cmd.OutOrStdout(), name)
			return err
		},
	}
}

func newServeCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "serve",
		Short: "Run the HTTP server",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}
			pool, err := pgxpool.New(cmd.Context(), cfg.DatabaseURL)
			if err != nil {
				return fmt.Errorf("connecting to database: %w", err)
			}
			defer pool.Close()

			store := pg.NewAppStore(pg.New(pool))
			emailScanner := scanner.New(nil)
			sessionManager := scs.New()
			sessionManager.Lifetime = 24 * time.Hour

			logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
			server := &http.Server{
				Addr: cfg.HTTPAddr,
				Handler: web.NewRouterWithDeps(web.Deps{
					Scanner:        emailScanner,
					Auth:           auth.NewService(store, time.Duration(cfg.AuthTokenTTL)*time.Minute),
					Domains:        domains.NewService(store, emailScanner),
					Sender:         notifier.NewLettermintSender(cfg.LettermintBaseURL, cfg.LettermintToken, cfg.LettermintFrom),
					SessionManager: sessionManager,
					PublicBaseURL:  cfg.PublicBaseURL,
				}),
				ReadHeaderTimeout: 5 * time.Second,
			}

			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()

			errCh := make(chan error, 1)
			go func() {
				logger.Info("http server starting", slog.String("addr", cfg.HTTPAddr))
				if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
					errCh <- err
				}
				close(errCh)
			}()

			select {
			case <-ctx.Done():
				shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				if err := server.Shutdown(shutdownCtx); err != nil {
					return fmt.Errorf("shutting down http server: %w", err)
				}
				return nil
			case err := <-errCh:
				if err != nil {
					return fmt.Errorf("running http server: %w", err)
				}
				return nil
			}
		},
	}
}
