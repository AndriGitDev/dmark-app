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
	r.Get("/", templ.Handler(views.Home()).ServeHTTP)
	return r
}
