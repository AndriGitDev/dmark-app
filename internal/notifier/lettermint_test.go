package notifier

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLettermintSenderSendsMagicLinkEmail(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/send" {
			t.Fatalf("path = %q, want /send", r.URL.Path)
		}
		if token := r.Header.Get("x-lettermint-token"); token != "secret" {
			t.Fatalf("token = %q, want secret", token)
		}
		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decoding request body: %v", err)
		}
		if body["from"] != "Meil <login@example.com>" || body["subject"] != "Sign in to Meil" {
			t.Fatalf("unexpected request body: %#v", body)
		}
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()

	sender := NewLettermintSender(server.URL, "secret", "Meil <login@example.com>")
	if err := sender.Send(context.Background(), Email{To: "person@example.com", Subject: "Sign in to Meil", Text: "link"}); err != nil {
		t.Fatalf("Send returned error: %v", err)
	}
}
