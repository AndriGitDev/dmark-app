package notifier

import (
	"strings"
	"testing"

	"github.com/AndriGitDev/meil/internal/drift"
)

func TestBuildDigestEmail(t *testing.T) {
	t.Parallel()

	email := BuildDigestEmail(Digest{
		Recipient: "ops@example.com",
		Items: []DigestItem{
			{Domain: "b.example", Event: drift.Event{CheckName: "SPF", Severity: drift.SeverityWarn, Message: "SPF changed."}},
			{Domain: "a.example", Event: drift.Event{CheckName: "DMARC", Severity: drift.SeverityCritical, Message: "DMARC relaxed."}},
		},
	})
	if email.To != "ops@example.com" || email.Subject != "RecordMaster digest: 2 event(s)" {
		t.Fatalf("unexpected digest email: %+v", email)
	}
	first := strings.Index(email.Text, "a.example")
	second := strings.Index(email.Text, "b.example")
	if first == -1 || second == -1 || first > second {
		t.Fatalf("digest did not sort critical event first: %s", email.Text)
	}
}

func TestBuildDigestEmailEmpty(t *testing.T) {
	t.Parallel()

	email := BuildDigestEmail(Digest{Recipient: "ops@example.com"})
	if !strings.Contains(email.Text, "No drift events") {
		t.Fatalf("empty digest text = %q", email.Text)
	}
}
