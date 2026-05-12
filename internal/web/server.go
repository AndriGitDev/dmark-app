// Package web wires HTTP routes and middleware.
package web

import (
	"net/http"

	"github.com/a-h/templ"
	"github.com/go-chi/chi/v5"

	"github.com/AndriGitDev/meil/internal/web/views"
)

func NewRouter() http.Handler {
	r := chi.NewRouter()
	r.Handle("/assets/*", http.StripPrefix("/assets/", http.FileServer(http.Dir("assets"))))
	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	r.Get("/", templ.Handler(views.Home()).ServeHTTP)
	return r
}
