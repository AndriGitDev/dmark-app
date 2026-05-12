package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/AndriGitDev/meil/internal/config"
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
	root.AddCommand(stubCommand("worker"))
	root.AddCommand(stubCommand("ingest"))
	root.AddCommand(stubCommand("migrate"))
	root.AddCommand(stubCommand("scan"))

	return root
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

			logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
			server := &http.Server{
				Addr:              cfg.HTTPAddr,
				Handler:           web.NewRouter(),
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
