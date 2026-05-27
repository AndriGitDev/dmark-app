package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/alexedwards/scs/v2"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/riverqueue/river"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivermigrate"
	"github.com/spf13/cobra"

	"github.com/AndriGitDev/meil/internal/attribution"
	"github.com/AndriGitDev/meil/internal/auth"
	"github.com/AndriGitDev/meil/internal/config"
	"github.com/AndriGitDev/meil/internal/domains"
	"github.com/AndriGitDev/meil/internal/ingest"
	"github.com/AndriGitDev/meil/internal/jobs"
	"github.com/AndriGitDev/meil/internal/notifier"
	"github.com/AndriGitDev/meil/internal/scanner"
	"github.com/AndriGitDev/meil/internal/store/ch"
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
		Short: "Dmark DMARC diagnostics and remediation",
	}

	root.AddCommand(newServeCommand())
	root.AddCommand(newHealthcheckCommand())
	root.AddCommand(newWorkerCommand())
	root.AddCommand(newIngestCommand())
	root.AddCommand(newMigrateCommand())
	root.AddCommand(newScanCommand())

	return root
}

func newWorkerCommand() *cobra.Command {
	var listJobs bool
	cmd := &cobra.Command{
		Use:   "worker",
		Short: "Run background job workers",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}
			if err := cfg.Validate(); err != nil {
				return fmt.Errorf("validating config: %w", err)
			}
			registered := []string{jobs.ScanDomainKind, jobs.RefreshEnrichmentKind}
			if listJobs {
				for _, kind := range registered {
					if _, err := fmt.Fprintln(cmd.OutOrStdout(), kind); err != nil {
						return err
					}
				}
				return nil
			}
			pool, err := pgxpool.New(cmd.Context(), cfg.DatabaseURL)
			if err != nil {
				return fmt.Errorf("connecting to database: %w", err)
			}
			defer pool.Close()

			store := pg.NewAppStore(pg.New(pool))
			enrichment := attribution.NewService(attribution.StaticEnricher{}, attribution.DefaultClassifier(), attribution.DefaultCachePolicy())
			workers, err := jobs.NewWorkers(store, scanner.New(nil), enrichment)
			if err != nil {
				return fmt.Errorf("registering workers: %w", err)
			}
			client, err := river.NewClient(riverpgxv5.New(pool), &river.Config{
				Logger: slog.New(slog.NewJSONHandler(os.Stdout, nil)),
				Queues: map[string]river.QueueConfig{
					jobs.ScanQueue: {MaxWorkers: 4},
				},
				Workers: workers,
			})
			if err != nil {
				return fmt.Errorf("creating River client: %w", err)
			}
			if err := client.Start(cmd.Context()); err != nil {
				return fmt.Errorf("starting River worker: %w", err)
			}
			if _, err := fmt.Fprintf(cmd.OutOrStdout(), "worker running job kinds: %s\n", strings.Join(registered, ", ")); err != nil {
				return err
			}
			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			<-ctx.Done()

			shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			if err := client.Stop(shutdownCtx); err != nil {
				return fmt.Errorf("stopping River worker: %w", err)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&listJobs, "list-jobs", false, "list registered background job kinds and exit")
	return cmd
}

func newMigrateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "migrate [postgres|clickhouse] [up|down|status]",
		Short: "Run database migrations",
		Args:  cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}
			if err := cfg.Validate(); err != nil {
				return fmt.Errorf("validating config: %w", err)
			}
			target, direction, err := migrationTarget(args)
			if err != nil {
				return err
			}
			if target == "clickhouse" {
				return runClickHouseMigration(cmd.Context(), cfg, direction, cmd.OutOrStdout())
			}
			return runPostgresMigration(cmd.Context(), cfg, direction, cmd.OutOrStdout())
		},
	}
	return cmd
}

func migrationTarget(args []string) (string, string, error) {
	if len(args) == 1 {
		return "postgres", args[0], nil
	}
	switch args[0] {
	case "postgres", "clickhouse":
		return args[0], args[1], nil
	default:
		return "", "", fmt.Errorf("unknown migration target %q", args[0])
	}
}

func migrationDir(dir string) string {
	if _, err := os.Stat(dir); err == nil {
		return dir
	}
	if filepath.IsAbs(dir) {
		return dir
	}
	return filepath.Join(string(os.PathSeparator), dir)
}

func runPostgresMigration(ctx context.Context, cfg config.Config, direction string, out io.Writer) error {
	db, err := sql.Open("pgx", cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("opening database: %w", err)
	}
	defer func() { _ = db.Close() }()
	if err := goose.SetDialect("postgres"); err != nil {
		return fmt.Errorf("setting migration dialect: %w", err)
	}
	postgresMigrations := migrationDir("migrations/postgres")

	switch direction {
	case "up":
		if err := goose.Up(db, postgresMigrations); err != nil {
			return err
		}
		return runRiverMigration(ctx, cfg, rivermigrate.DirectionUp, out)
	case "down":
		if err := runRiverMigration(ctx, cfg, rivermigrate.DirectionDown, out); err != nil {
			return err
		}
		return goose.Down(db, postgresMigrations)
	case "status":
		if err := goose.Status(db, postgresMigrations); err != nil {
			return err
		}
		return printRiverMigrationStatus(ctx, cfg, out)
	default:
		return fmt.Errorf("unknown migration direction %q", direction)
	}
}

func runRiverMigration(ctx context.Context, cfg config.Config, direction rivermigrate.Direction, out io.Writer) error {
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("connecting to database for River migrations: %w", err)
	}
	defer pool.Close()

	migrator, err := rivermigrate.New(riverpgxv5.New(pool), nil)
	if err != nil {
		return fmt.Errorf("creating River migrator: %w", err)
	}
	result, err := migrator.Migrate(ctx, direction, nil)
	if err != nil {
		return fmt.Errorf("running River migrations: %w", err)
	}
	for _, version := range result.Versions {
		if _, err := fmt.Fprintf(out, "river %s %d\n", result.Direction, version.Version); err != nil {
			return err
		}
	}
	return nil
}

func printRiverMigrationStatus(ctx context.Context, cfg config.Config, out io.Writer) error {
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("connecting to database for River migration status: %w", err)
	}
	defer pool.Close()

	migrator, err := rivermigrate.New(riverpgxv5.New(pool), nil)
	if err != nil {
		return fmt.Errorf("creating River migrator: %w", err)
	}
	versions, err := migrator.ExistingVersions(ctx)
	if err != nil {
		return fmt.Errorf("reading River migration status: %w", err)
	}
	applied := make([]string, 0, len(versions))
	for _, version := range versions {
		applied = append(applied, fmt.Sprintf("%d", version.Version))
	}
	_, err = fmt.Fprintf(out, "River migrations: %s\n", strings.Join(applied, ", "))
	return err
}

func runClickHouseMigration(ctx context.Context, cfg config.Config, direction string, out io.Writer) error {
	migrationConfig := ch.MigrationConfig{
		URL:      cfg.ClickHouseURL,
		Database: cfg.ClickHouseDatabase,
		User:     cfg.ClickHouseUser,
		Password: cfg.ClickHousePassword,
		Dir:      migrationDir("migrations/clickhouse"),
	}
	switch direction {
	case "up":
		return ch.RunMigrations(ctx, migrationConfig)
	case "status":
		status, err := ch.Status(ctx, migrationConfig)
		if err != nil {
			return err
		}
		_, err = fmt.Fprintf(out, "ClickHouse database: %s\nTables: %s\n", status.Database, strings.Join(status.Tables, ", "))
		return err
	case "down":
		return fmt.Errorf("ClickHouse down migrations are not implemented")
	default:
		return fmt.Errorf("unknown migration direction %q", direction)
	}
}

func newIngestCommand() *cobra.Command {
	var jsonOutput bool
	var maxBytes int64
	cmd := &cobra.Command{
		Use:   "ingest <report-file>...",
		Short: "Parse DMARC aggregate report payloads",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			payload := ingestSummaryPayload{Files: make([]ingestFileSummary, 0, len(args))}
			for _, path := range args {
				data, sourceName, err := readReportPayload(cmd, path)
				if err != nil {
					return fmt.Errorf("reading %s: %w", path, err)
				}
				reports, err := ingest.ParseAggregatePayload(cmd.Context(), sourceName, data, maxBytes)
				if err != nil {
					return fmt.Errorf("parsing %s: %w", path, err)
				}
				fileSummary := ingestFileSummary{Path: path, Reports: make([]ingestReportSummary, 0, len(reports))}
				for _, report := range reports {
					records, messages := reportRecordTotals(report)
					fileSummary.Reports = append(fileSummary.Reports, ingestReportSummary{
						ReportID:      report.ReportID,
						OrgName:       report.OrgName,
						Domain:        report.Domain,
						BeginUnix:     report.BeginUnix,
						EndUnix:       report.EndUnix,
						Records:       records,
						Messages:      messages,
						ContentHash:   report.ContentHash,
						SourceFile:    report.SourceFile,
						AuthResults:   len(report.AuthResults),
						PolicyApplied: report.Policy.P,
					})
					payload.TotalReports++
					payload.TotalRecords += records
					payload.TotalMessages += messages
				}
				payload.Files = append(payload.Files, fileSummary)
			}
			if jsonOutput {
				encoder := json.NewEncoder(cmd.OutOrStdout())
				encoder.SetIndent("", "  ")
				return encoder.Encode(payload)
			}
			return printIngestSummary(cmd.OutOrStdout(), payload)
		},
	}
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "print parsed report metadata as JSON")
	cmd.Flags().Int64Var(&maxBytes, "max-bytes", ingest.DefaultMaxReportBytes, "maximum decompressed bytes per report payload")
	return cmd
}

func readReportPayload(cmd *cobra.Command, path string) ([]byte, string, error) {
	if path == "-" {
		data, err := io.ReadAll(cmd.InOrStdin())
		return data, "stdin.xml", err
	}
	// #nosec G304 -- ingest is an operator CLI that intentionally reads user-supplied report paths.
	data, err := os.ReadFile(path)
	return data, path, err
}

type ingestSummaryPayload struct {
	Files         []ingestFileSummary `json:"files"`
	TotalReports  int                 `json:"total_reports"`
	TotalRecords  int                 `json:"total_records"`
	TotalMessages int                 `json:"total_messages"`
}

type ingestFileSummary struct {
	Path    string                `json:"path"`
	Reports []ingestReportSummary `json:"reports"`
}

type ingestReportSummary struct {
	ReportID      string `json:"report_id"`
	OrgName       string `json:"org_name"`
	Domain        string `json:"domain"`
	BeginUnix     int64  `json:"begin_unix"`
	EndUnix       int64  `json:"end_unix"`
	Records       int    `json:"records"`
	Messages      int    `json:"messages"`
	ContentHash   string `json:"content_hash"`
	SourceFile    string `json:"source_file"`
	AuthResults   int    `json:"auth_results"`
	PolicyApplied string `json:"policy_applied"`
}

func reportRecordTotals(report ingest.AggregateReport) (int, int) {
	messages := 0
	for _, record := range report.Records {
		messages += record.Count
	}
	return len(report.Records), messages
}

func printIngestSummary(w io.Writer, payload ingestSummaryPayload) error {
	if _, err := fmt.Fprintf(w, "Reports: %d\nRecords: %d\nMessages: %d\n", payload.TotalReports, payload.TotalRecords, payload.TotalMessages); err != nil {
		return err
	}
	for _, file := range payload.Files {
		if _, err := fmt.Fprintf(w, "\nFile: %s\n", file.Path); err != nil {
			return err
		}
		for _, report := range file.Reports {
			if _, err := fmt.Fprintf(w, "%s: %s from %s, %d records, %d messages, policy %s\n", report.ReportID, report.Domain, report.OrgName, report.Records, report.Messages, report.PolicyApplied); err != nil {
				return err
			}
		}
	}
	return nil
}

func newScanCommand() *cobra.Command {
	var jsonOutput bool
	var selectors []string
	var providers []string
	cmd := &cobra.Command{
		Use:   "scan <domain>",
		Short: "Run a one-off record check",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			for _, provider := range providers {
				selectors = append(selectors, scanner.ProviderDKIMSelectors(provider)...)
			}
			emailScanner := scanner.New(nil)
			if len(selectors) > 0 {
				emailScanner = scanner.NewWithSelectors(nil, selectors)
			}
			result, err := emailScanner.Scan(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			if jsonOutput {
				return printScanResultJSON(cmd.OutOrStdout(), result)
			}
			return printScanResult(cmd.OutOrStdout(), result)
		},
	}
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "print scan result as JSON")
	cmd.Flags().StringArrayVar(&selectors, "selector", nil, "custom DKIM selector to probe; may be repeated")
	cmd.Flags().StringArrayVar(&providers, "provider", nil, "DKIM provider preset to include; may be repeated")
	return cmd
}

func printScanResult(w io.Writer, result scanner.Result) error {
	summary := scanner.Summarize(result)
	if _, err := fmt.Fprintf(w, "Domain: %s\n", result.Domain); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "Score: %d (%s)\nTop issue: %s\n", summary.Score, summary.Status, summary.TopIssue); err != nil {
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

func printScanResultJSON(w io.Writer, result scanner.Result) error {
	summary := scanner.Summarize(result)
	payload := scanJSONPayload{
		Domain:     result.Domain,
		CheckedAt:  result.CheckedAt.Format(time.RFC3339),
		DurationMS: result.Duration.Milliseconds(),
		Summary: scanJSONSummary{
			Score:       summary.Score,
			Status:      summary.Status,
			TopIssue:    summary.TopIssue,
			Remediation: summary.Remediation,
		},
		Checks: []scanJSONCheck{
			newScanJSONCheck(result.SPF),
			newScanJSONCheck(result.DMARC),
			newScanJSONCheck(result.BIMI),
			newScanJSONCheck(result.DKIM),
		},
	}
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(payload)
}

type scanJSONPayload struct {
	Domain     string          `json:"domain"`
	CheckedAt  string          `json:"checked_at"`
	DurationMS int64           `json:"duration_ms"`
	Summary    scanJSONSummary `json:"summary"`
	Checks     []scanJSONCheck `json:"checks"`
}

type scanJSONSummary struct {
	Score       int            `json:"score"`
	Status      scanner.Status `json:"status"`
	TopIssue    string         `json:"top_issue"`
	Remediation string         `json:"remediation"`
}

type scanJSONCheck struct {
	Name              string                   `json:"name"`
	Status            scanner.Status           `json:"status"`
	Message           string                   `json:"message"`
	Record            string                   `json:"record,omitempty"`
	SourceName        string                   `json:"source_name,omitempty"`
	LookupDurationMS  int64                    `json:"lookup_duration_ms"`
	ErrorClass        string                   `json:"error_class,omitempty"`
	SelectorSummaries []scanner.SelectorResult `json:"selectors,omitempty"`
}

func newScanJSONCheck(result scanner.CheckResult) scanJSONCheck {
	return scanJSONCheck{
		Name:              result.Name,
		Status:            result.Status,
		Message:           result.Message,
		Record:            result.Record,
		SourceName:        result.SourceName,
		LookupDurationMS:  result.LookupDuration.Milliseconds(),
		ErrorClass:        result.ErrorClass,
		SelectorSummaries: result.Selectors,
	}
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

func newServeCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "serve",
		Short: "Run the HTTP server",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return fmt.Errorf("loading config: %w", err)
			}
			if err := cfg.Validate(); err != nil {
				return fmt.Errorf("validating config: %w", err)
			}
			logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
			logger.Info("running startup migrations")
			if err := runPostgresMigration(cmd.Context(), cfg, "up", io.Discard); err != nil {
				return fmt.Errorf("running startup migrations: %w", err)
			}
			logger.Info("startup migrations complete")

			pool, err := pgxpool.New(cmd.Context(), cfg.DatabaseURL)
			if err != nil {
				return fmt.Errorf("connecting to database: %w", err)
			}
			defer pool.Close()

			store := pg.NewAppStore(pg.New(pool))
			emailScanner := scanner.New(nil)
			sessionManager := scs.New()
			sessionManager.Lifetime = 24 * time.Hour

			server := &http.Server{
				Addr: cfg.HTTPAddr,
				Handler: web.NewRouterWithDeps(web.Deps{
					Scanner:               emailScanner,
					APIKeys:               store,
					APIDomains:            store,
					APIDomainCreator:      store,
					APIDomainChecker:      store,
					APIDomainHistory:      store,
					APIDomainDrift:        store,
					APIReporter:           store,
					UserReporter:          store,
					AuditClaims:           store,
					APIWebhooks:           store,
					Billing:               store,
					BillingCancellation:   store,
					BillingWebhookSecret:  cfg.BillingWebhookSecret,
					APIKeyManagement:      store,
					APITeams:              store,
					APIPrivacy:            store,
					APIAdmin:              store,
					APIStatus:             store,
					PrivacyDeletion:       store,
					Auth:                  auth.NewService(store, time.Duration(cfg.AuthTokenTTL)*time.Minute),
					Domains:               domains.NewService(store, emailScanner),
					Sender:                notifier.NewLettermintSender(cfg.LettermintBaseURL, cfg.LettermintToken, cfg.LettermintFrom),
					SessionManager:        sessionManager,
					PublicBaseURL:         cfg.PublicBaseURL,
					PublicChecksPerMinute: cfg.PublicChecksPerMinute,
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
