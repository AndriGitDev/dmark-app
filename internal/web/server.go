// Package web wires HTTP routes and middleware.
package web

import (
	"context"
	"net/http"
	"os"
	"path/filepath"

	"github.com/a-h/templ"
	"github.com/go-chi/chi/v5"

	"github.com/AndriGitDev/meil/internal/scanner"
	"github.com/AndriGitDev/meil/internal/web/views"
)

type EmailScanner interface {
	Scan(ctx context.Context, domain string) (scanner.Result, error)
}

func NewRouter() http.Handler {
	return NewRouterWithScanner(scanner.New(nil))
}

func NewRouterWithScanner(emailScanner EmailScanner) http.Handler {
	r := chi.NewRouter()
	assetsDir := os.Getenv("ASSETS_DIR")
	if assetsDir == "" {
		assetsDir = "assets"
	}
	r.Get("/assets/css/marketing-20260512.css", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join(assetsDir, "css", "output.css"))
	})
	r.Handle("/assets/*", http.StripPrefix("/assets/", http.FileServer(http.Dir(assetsDir))))
	r.Post("/check", func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, 4096)
		if err := r.ParseForm(); err != nil {
			http.Error(w, "invalid form", http.StatusBadRequest)
			return
		}
		result, err := emailScanner.Scan(r.Context(), r.PostForm.Get("domain"))
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		templ.Handler(views.CheckResult(result)).ServeHTTP(w, r)
	})
	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	r.Get("/", templ.Handler(views.Home()).ServeHTTP)
	return r
}
