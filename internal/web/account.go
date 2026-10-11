package web

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"html"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/AndriGitDev/meil/internal/auth"
	"github.com/AndriGitDev/meil/internal/notifier"
	"github.com/AndriGitDev/meil/internal/workspace"
	"github.com/go-chi/chi/v5"
)

var accountTokenPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{43}$`)

func registerAccountRoutes(r chi.Router, deps Deps) {
	frontend := strings.TrimRight(deps.FrontendBaseURL, "/")
	if frontend == "" {
		frontend = "https://dmark.is"
	}
	limit := deps.LoginAttemptsPerMinute
	if limit <= 0 {
		limit = 6
	}
	limiter := newPublicRateLimiter(limit)
	sourceLimiter := newPublicRateLimiter(120)
	available := deps.Auth != nil && deps.Sender != nil && deps.Workspace != nil
	csrf := func(req *http.Request) string {
		token := deps.SessionManager.GetString(req.Context(), "account_csrf")
		if token == "" {
			raw := make([]byte, 32)
			if _, err := rand.Read(raw); err != nil {
				return ""
			}
			token = base64.RawURLEncoding.EncodeToString(raw)
			deps.SessionManager.Put(req.Context(), "account_csrf", token)
		}
		return token
	}
	respond := func(w http.ResponseWriter, status int, data any) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(data)
	}
	failure := func(w http.ResponseWriter, status int, code string) {
		respond(w, status, map[string]string{"code": code})
	}
	protected := func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, req *http.Request) {
			origin := req.Header.Get("Origin")
			allowed := origin == frontend
			if !allowed {
				failure(w, 403, "INVALID_ORIGIN")
				return
			}
			stored := deps.SessionManager.GetString(req.Context(), "account_csrf")
			supplied := req.Header.Get("X-CSRF-Token")
			if stored == "" || subtle.ConstantTimeCompare([]byte(stored), []byte(supplied)) != 1 {
				failure(w, 403, "INVALID_CSRF")
				return
			}
			next(w, req)
		}
	}
	decode := func(w http.ResponseWriter, req *http.Request, target any) bool {
		req.Body = http.MaxBytesReader(w, req.Body, 16000)
		decoder := json.NewDecoder(req.Body)
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(target); err != nil {
			failure(w, 400, "INVALID_REQUEST")
			return false
		}
		var extra any
		if err := decoder.Decode(&extra); err != nil && errors.Is(err, io.EOF) {
			return true
		}
		failure(w, 400, "INVALID_REQUEST")
		return false
	}
	r.Route("/api/account", func(r chi.Router) {
		r.Use(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
				w.Header().Set("Cache-Control", "no-store")
				w.Header().Set("Referrer-Policy", "no-referrer")
				next.ServeHTTP(w, req)
			})
		})
		r.Get("/me", func(w http.ResponseWriter, req *http.Request) {
			respond(w, 200, map[string]any{"authenticated": deps.SessionManager.GetString(req.Context(), "account_user_id") != "", "email": deps.SessionManager.GetString(req.Context(), "account_email"), "csrf": csrf(req), "available": available})
		})
		r.Post("/start", protected(func(w http.ResponseWriter, req *http.Request) {
			if !available {
				failure(w, 503, "ACCOUNT_UNAVAILABLE")
				return
			}
			var input struct {
				Email  string `json:"email"`
				Locale string `json:"locale"`
			}
			if !decode(w, req, &input) {
				return
			}
			email, err := auth.NormalizeEmail(input.Email)
			if err != nil || len(email) > 254 {
				failure(w, 400, "INVALID_EMAIL")
				return
			}
			// The proxy shares an upstream address: also cap each recipient and session.
			now := time.Now()
			source, _, splitErr := net.SplitHostPort(req.RemoteAddr)
			if splitErr != nil {
				source = req.RemoteAddr
			}
			if !limiter.allow("email:"+email, now) || !limiter.allow("session:"+deps.SessionManager.Token(req.Context()), now) || !sourceLimiter.allow(source, now) {
				failure(w, 429, "RATE_LIMITED")
				return
			}
			email, token, err := deps.Auth.StartLogin(req.Context(), email)
			if err != nil {
				failure(w, 503, "ACCOUNT_UNAVAILABLE")
				return
			}
			locale := "en"
			if input.Locale == "is" {
				locale = "is"
			}
			link := frontend + "/api/account/confirm/?" + url.Values{"token": {token}, "locale": {locale}}.Encode()
			subject := "Sign in to Dmark"
			text := "Open this link in the same browser where you requested it. It expires shortly and can be used once.\n\n" + link + "\n\nYour domain workspace remains free."
			if locale == "is" {
				subject = "Skrá inn í Dmark"
				text = "Opnaðu hlekkinn í sama vafra og þú óskaðir eftir honum. Hann gildir í stuttan tíma og aðeins einu sinni.\n\n" + link + "\n\nVinnusvæðið þitt er áfram ókeypis."
			}
			if err := deps.Sender.Send(req.Context(), notifier.Email{To: email, Subject: subject, Text: text, HTML: "<p>" + strings.ReplaceAll(strings.ReplaceAll(html.EscapeString(text), html.EscapeString(link), `<a href="`+html.EscapeString(link)+`">`+html.EscapeString(link)+`</a>`), "\n", "<br>") + "</p>"}); err != nil {
				failure(w, 503, "ACCOUNT_UNAVAILABLE")
				return
			}
			hash := sha256.Sum256([]byte(token))
			deps.SessionManager.Put(req.Context(), "account_pending_hash", base64.RawURLEncoding.EncodeToString(hash[:]))
			respond(w, 202, map[string]bool{"sent": true})
		}))
		r.Post("/finish", protected(func(w http.ResponseWriter, req *http.Request) {
			if !available {
				failure(w, 503, "ACCOUNT_UNAVAILABLE")
				return
			}
			var input struct {
				Token string `json:"token"`
			}
			if !decode(w, req, &input) {
				return
			}
			hash := sha256.Sum256([]byte(input.Token))
			pending := deps.SessionManager.GetString(req.Context(), "account_pending_hash")
			if !accountTokenPattern.MatchString(input.Token) || pending == "" || subtle.ConstantTimeCompare([]byte(pending), []byte(base64.RawURLEncoding.EncodeToString(hash[:]))) != 1 {
				failure(w, 400, "INVALID_TOKEN")
				return
			}
			login, err := deps.Auth.FinishLogin(req.Context(), input.Token)
			if err != nil {
				failure(w, 400, "INVALID_TOKEN")
				return
			}
			if err := deps.SessionManager.RenewToken(req.Context()); err != nil {
				failure(w, 500, "ACCOUNT_UNAVAILABLE")
				return
			}
			deps.SessionManager.Remove(req.Context(), "account_pending_hash")
			deps.SessionManager.Remove(req.Context(), "account_csrf")
			deps.SessionManager.Put(req.Context(), "account_user_id", login.UserID)
			deps.SessionManager.Put(req.Context(), "account_email", login.Email)
			respond(w, 200, map[string]any{"authenticated": true, "email": login.Email, "csrf": csrf(req), "available": true})
		}))
		r.Get("/workspace", func(w http.ResponseWriter, req *http.Request) {
			id := deps.SessionManager.GetString(req.Context(), "account_user_id")
			if id == "" {
				failure(w, 401, "AUTH_REQUIRED")
				return
			}
			if deps.Workspace == nil {
				failure(w, 503, "ACCOUNT_UNAVAILABLE")
				return
			}
			d, err := deps.Workspace.Load(req.Context(), id)
			if err != nil {
				failure(w, 503, "ACCOUNT_UNAVAILABLE")
				return
			}
			respond(w, 200, d)
		})
		r.Post("/workspace", protected(func(w http.ResponseWriter, req *http.Request) {
			id := deps.SessionManager.GetString(req.Context(), "account_user_id")
			if id == "" {
				failure(w, 401, "AUTH_REQUIRED")
				return
			}
			if deps.Workspace == nil {
				failure(w, 503, "ACCOUNT_UNAVAILABLE")
				return
			}
			var input workspace.Document
			if !decode(w, req, &input) {
				return
			}
			values, err := workspace.Validate(input.Domains)
			if err != nil || input.Version < 0 {
				failure(w, 400, "INVALID_WORKSPACE")
				return
			}
			d, err := deps.Workspace.Save(req.Context(), id, input.Version, values)
			if errors.Is(err, workspace.ErrConflict) {
				failure(w, 409, "WORKSPACE_CONFLICT")
				return
			}
			if err != nil {
				failure(w, 503, "ACCOUNT_UNAVAILABLE")
				return
			}
			respond(w, 200, d)
		}))
		r.Post("/logout", protected(func(w http.ResponseWriter, req *http.Request) {
			if err := deps.SessionManager.Destroy(req.Context()); err != nil {
				failure(w, 500, "ACCOUNT_UNAVAILABLE")
				return
			}
			respond(w, 200, map[string]bool{"authenticated": false})
		}))
	})
}
