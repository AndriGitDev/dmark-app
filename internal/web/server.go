// Package web wires HTTP routes and middleware.
package web

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/a-h/templ"
	"github.com/alexedwards/scs/v2"
	"github.com/go-chi/chi/v5"

	"github.com/AndriGitDev/meil/internal/auth"
	"github.com/AndriGitDev/meil/internal/domains"
	"github.com/AndriGitDev/meil/internal/notifier"
	"github.com/AndriGitDev/meil/internal/scanner"
	"github.com/AndriGitDev/meil/internal/web/views"
)

type EmailScanner interface {
	Scan(ctx context.Context, domain string) (scanner.Result, error)
}

type Deps struct {
	Scanner        EmailScanner
	Auth           *auth.Service
	Domains        *domains.Service
	Sender         notifier.EmailSender
	SessionManager *scs.SessionManager
	PublicBaseURL  string
}

func NewRouter() http.Handler {
	return NewRouterWithScanner(scanner.New(nil))
}

func NewRouterWithScanner(emailScanner EmailScanner) http.Handler {
	return NewRouterWithDeps(Deps{Scanner: emailScanner})
}

func NewRouterWithDeps(deps Deps) http.Handler {
	if deps.Scanner == nil {
		deps.Scanner = scanner.New(nil)
	}
	if deps.SessionManager == nil {
		deps.SessionManager = scs.New()
		deps.SessionManager.Lifetime = 24 * time.Hour
	}
	if deps.PublicBaseURL == "" {
		deps.PublicBaseURL = "http://localhost:8080"
	}

	r := chi.NewRouter()
	r.Use(deps.SessionManager.LoadAndSave)
	assetsDir := os.Getenv("ASSETS_DIR")
	if assetsDir == "" {
		assetsDir = "assets"
	}
	r.Get("/assets/css/marketing-20260512.css", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, filepath.Join(assetsDir, "css", "output.css"))
	})
	r.Handle("/assets/*", http.StripPrefix("/assets/", http.FileServer(http.Dir(assetsDir))))
	r.Get("/login", templ.Handler(views.Login()).ServeHTTP)
	r.Post("/login", func(w http.ResponseWriter, r *http.Request) {
		if deps.Auth == nil || deps.Sender == nil {
			http.Error(w, "login is not configured", http.StatusServiceUnavailable)
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 4096)
		if err := r.ParseForm(); err != nil {
			http.Error(w, "invalid form", http.StatusBadRequest)
			return
		}
		email, token, err := deps.Auth.StartLogin(r.Context(), r.PostForm.Get("email"))
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		link := deps.PublicBaseURL + "/auth/callback?token=" + token
		if err := deps.Sender.Send(r.Context(), notifier.Email{
			To:      email,
			Subject: "Sign in to Meil",
			Text:    fmt.Sprintf("Sign in to Meil: %s", link),
			HTML:    fmt.Sprintf(`<p>Sign in to Meil:</p><p><a href="%s">Open your magic link</a></p>`, link),
		}); err != nil {
			http.Error(w, "sending login email failed", http.StatusBadGateway)
			return
		}
		templ.Handler(views.LoginSent(email)).ServeHTTP(w, r)
	})
	r.Get("/auth/callback", func(w http.ResponseWriter, r *http.Request) {
		if deps.Auth == nil {
			http.Error(w, "login is not configured", http.StatusServiceUnavailable)
			return
		}
		login, err := deps.Auth.FinishLogin(r.Context(), r.URL.Query().Get("token"))
		if err != nil {
			http.Error(w, "invalid or expired login link", http.StatusBadRequest)
			return
		}
		deps.SessionManager.Put(r.Context(), "user_id", login.UserID)
		deps.SessionManager.Put(r.Context(), "user_email", login.Email)
		http.Redirect(w, r, "/app/domains", http.StatusSeeOther)
	})
	r.Post("/logout", func(w http.ResponseWriter, r *http.Request) {
		_ = deps.SessionManager.Destroy(r.Context())
		http.Redirect(w, r, "/", http.StatusSeeOther)
	})
	r.Post("/check", func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, 4096)
		if err := r.ParseForm(); err != nil {
			http.Error(w, "invalid form", http.StatusBadRequest)
			return
		}
		result, err := deps.Scanner.Scan(r.Context(), r.PostForm.Get("domain"))
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		templ.Handler(views.CheckResult(result)).ServeHTTP(w, r)
	})
	r.Route("/app", func(r chi.Router) {
		r.Use(requireLogin(deps.SessionManager))
		r.Get("/domains", func(w http.ResponseWriter, r *http.Request) {
			if deps.Domains == nil {
				http.Error(w, "domains are not configured", http.StatusServiceUnavailable)
				return
			}
			saved, err := deps.Domains.ListDomains(r.Context(), currentUserID(deps.SessionManager, r))
			if err != nil {
				http.Error(w, "loading domains failed", http.StatusInternalServerError)
				return
			}
			templ.Handler(views.AppDomains(currentUserEmail(deps.SessionManager, r), saved)).ServeHTTP(w, r)
		})
		r.Post("/domains", func(w http.ResponseWriter, r *http.Request) {
			if deps.Domains == nil {
				http.Error(w, "domains are not configured", http.StatusServiceUnavailable)
				return
			}
			r.Body = http.MaxBytesReader(w, r.Body, 4096)
			if err := r.ParseForm(); err != nil {
				http.Error(w, "invalid form", http.StatusBadRequest)
				return
			}
			if err := deps.Domains.AddDomain(r.Context(), currentUserID(deps.SessionManager, r), r.PostForm.Get("domain")); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			http.Redirect(w, r, "/app/domains", http.StatusSeeOther)
		})
		r.Post("/domains/{id}/check", func(w http.ResponseWriter, r *http.Request) {
			if err := deps.Domains.ScanDomain(r.Context(), currentUserID(deps.SessionManager, r), chi.URLParam(r, "id")); err != nil {
				http.Error(w, "checking domain failed", http.StatusBadRequest)
				return
			}
			http.Redirect(w, r, "/app/domains", http.StatusSeeOther)
		})
		r.Post("/domains/{id}/delete", func(w http.ResponseWriter, r *http.Request) {
			if err := deps.Domains.DeleteDomain(r.Context(), currentUserID(deps.SessionManager, r), chi.URLParam(r, "id")); err != nil {
				http.Error(w, "deleting domain failed", http.StatusBadRequest)
				return
			}
			http.Redirect(w, r, "/app/domains", http.StatusSeeOther)
		})
	})
	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})
	r.Get("/", templ.Handler(views.Home()).ServeHTTP)
	return r
}

func requireLogin(sessionManager *scs.SessionManager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if currentUserID(sessionManager, r) == "" {
				http.Redirect(w, r, "/login", http.StatusSeeOther)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func currentUserID(sessionManager *scs.SessionManager, r *http.Request) string {
	return sessionManager.GetString(r.Context(), "user_id")
}

func currentUserEmail(sessionManager *scs.SessionManager, r *http.Request) string {
	return sessionManager.GetString(r.Context(), "user_email")
}
