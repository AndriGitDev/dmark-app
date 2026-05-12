// Package web wires HTTP routes and middleware.
package web

import (
	"net/http"
	"os"
	"path/filepath"

	"github.com/a-h/templ"
	"github.com/go-chi/chi/v5"

	"github.com/AndriGitDev/meil/internal/web/views"
)

func NewRouter() http.Handler {
	r := chi.NewRouter()
	assetsDir := os.Getenv("ASSETS_DIR")
	if assetsDir == "" {
		assetsDir = "assets"
	}
	r.Get("/assets/css/marketing-20260512.css", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join(assetsDir, "css", "output.css"))
	})
	r.Handle("/assets/*", http.StripPrefix("/assets/", http.FileServer(http.Dir(assetsDir))))
	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	r.Get("/", templ.Handler(views.Home()).ServeHTTP)
	return r
}
