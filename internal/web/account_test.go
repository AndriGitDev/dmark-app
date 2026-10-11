package web

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/AndriGitDev/meil/internal/auth"
	"github.com/AndriGitDev/meil/internal/notifier"
	"github.com/AndriGitDev/meil/internal/workspace"
)

type accountAuthFake struct {
	hash     []byte
	consumed bool
}

func (s *accountAuthFake) UpsertUserByEmail(context.Context, string) (auth.User, error) {
	return auth.User{ID: "user1", Email: "owner@example.com"}, nil
}

func (s *accountAuthFake) CreateLoginToken(_ context.Context, _ string, hash []byte, _ time.Time) error {
	s.hash = hash
	s.consumed = false
	return nil
}

func (s *accountAuthFake) ConsumeLoginToken(_ context.Context, hash []byte) (auth.LoginToken, error) {
	if s.consumed || !bytes.Equal(hash, s.hash) {
		return auth.LoginToken{}, auth.ErrInvalidToken
	}
	s.consumed = true
	return auth.LoginToken{UserID: "user1", Email: "owner@example.com"}, nil
}

type accountSenderFake struct{ messages []notifier.Email }

func (s *accountSenderFake) Send(_ context.Context, email notifier.Email) error {
	s.messages = append(s.messages, email)
	return nil
}

type accountWorkspaceFake struct{ docs map[string]workspace.Document }

func (s *accountWorkspaceFake) Load(_ context.Context, id string) (workspace.Document, error) {
	d, ok := s.docs[id]
	if !ok {
		d = workspace.Document{Domains: []workspace.Domain{}}
	}
	return d, nil
}

func (s *accountWorkspaceFake) Save(_ context.Context, id string, version int64, domains []workspace.Domain) (workspace.Document, error) {
	d := s.docs[id]
	if d.Version != version {
		return d, workspace.ErrConflict
	}
	d = workspace.Document{Domains: domains, Version: version + 1}
	s.docs[id] = d
	return d, nil
}

func TestFreeAccountFlow(t *testing.T) {
	authStore := &accountAuthFake{}
	sender := &accountSenderFake{}
	store := &accountWorkspaceFake{docs: map[string]workspace.Document{}}
	handler := NewRouterWithDeps(Deps{Auth: auth.NewService(authStore, time.Minute), Sender: sender, Workspace: store, FrontendBaseURL: "https://dmark.is"})
	var cookie *http.Cookie
	var csrf string
	call := func(method, path, body, origin string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, "https://backend.example/api/account/"+path, strings.NewReader(body))
		if cookie != nil {
			req.AddCookie(cookie)
		}
		req.Header.Set("Origin", origin)
		req.Header.Set("X-CSRF-Token", csrf)
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, req)
		for _, c := range rr.Result().Cookies() {
			if c.Name == "session" {
				cookie = c
			}
		}
		return rr
	}
	me := call("GET", "me", "", "")
	var data map[string]any
	if err := json.Unmarshal(me.Body.Bytes(), &data); err != nil {
		t.Fatal(err)
	}
	csrf = data["csrf"].(string)
	if len(csrf) != 43 || data["authenticated"] != false {
		t.Fatal(data)
	}
	if rr := call("GET", "workspace", "", ""); rr.Code != 401 {
		t.Fatal(rr.Code)
	}
	if rr := call("POST", "start", `{"email":"owner@example.com"}`, "https://evil.example"); rr.Code != 403 || len(sender.messages) != 0 {
		t.Fatal(rr.Code)
	}
	goodCSRF := csrf
	csrf = "wrong"
	if rr := call("POST", "start", `{"email":"owner@example.com"}`, "https://dmark.is"); rr.Code != 403 {
		t.Fatal(rr.Code)
	}
	csrf = goodCSRF
	if rr := call("POST", "start", `{"email":"owner@example.com","locale":"en"}`, "https://dmark.is"); rr.Code != 202 {
		t.Fatal(rr.Code, rr.Body.String())
	}
	var token string
	for _, line := range strings.Split(sender.messages[0].Text, "\n") {
		if strings.HasPrefix(line, "https://") {
			u, err := url.Parse(line)
			if err != nil {
				t.Fatal(err)
			}
			token = u.Query().Get("token")
		}
	}
	if len(token) != 43 {
		t.Fatal("missing login link")
	}
	expected := sha256.Sum256([]byte(token))
	if !bytes.Equal(expected[:], authStore.hash) {
		t.Fatal("token not hashed")
	}
	oldCookie := cookie
	cookie = nil
	other := call("GET", "me", "", "")
	_ = json.Unmarshal(other.Body.Bytes(), &data)
	csrf = data["csrf"].(string)
	if rr := call("POST", "finish", `{"token":"`+token+`"}`, "https://dmark.is"); rr.Code != 400 || authStore.consumed {
		t.Fatal("other browser consumed login")
	}
	cookie = oldCookie
	csrf = goodCSRF
	rr := call("POST", "finish", `{"token":"`+token+`"}`, "https://dmark.is")
	if rr.Code != 200 {
		t.Fatal(rr.Code, rr.Body.String())
	}
	if cookie.Value == oldCookie.Value {
		t.Fatal("session not rotated")
	}
	_ = json.Unmarshal(rr.Body.Bytes(), &data)
	csrf = data["csrf"].(string)
	if rr := call("POST", "finish", `{"token":"`+token+`"}`, "https://dmark.is"); rr.Code != 400 {
		t.Fatal("reused token")
	}
	body := `{"version":0,"domains":[{"domain":"example.com","completed":["spf"],"selector":"s1"}]}`
	if rr := call("POST", "workspace", body, "https://dmark.is"); rr.Code != 200 {
		t.Fatal(rr.Code, rr.Body.String())
	}
	if rr := call("POST", "workspace", body, "https://dmark.is"); rr.Code != 409 {
		t.Fatal("stale save accepted", rr.Code)
	}
	rr = call("GET", "workspace", "", "")
	var doc workspace.Document
	_ = json.Unmarshal(rr.Body.Bytes(), &doc)
	if rr.Code != 200 || doc.Version != 1 || len(doc.Domains) != 1 || doc.Domains[0].Selector != "s1" {
		t.Fatal(rr.Code, doc)
	}
	if rr := call("POST", "workspace", `{"version":1,"domains":[{"domain":"example.com","completed":["unknown"]}]}`, "https://dmark.is"); rr.Code != 400 {
		t.Fatal(rr.Code)
	}
	if _, err := store.Load(context.Background(), "other-user"); err != nil && !errors.Is(err, workspace.ErrConflict) {
		t.Fatal(err)
	}
	if len(store.docs["other-user"].Domains) != 0 {
		t.Fatal("cross-user access")
	}
	if rr := call("POST", "logout", "", "https://dmark.is"); rr.Code != 200 {
		t.Fatal(rr.Code)
	}
	if rr := call("GET", "workspace", "", ""); rr.Code != 401 {
		t.Fatal("logout failed", rr.Code)
	}
}
