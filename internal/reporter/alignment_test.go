package reporter

import (
	"strings"
	"testing"

	"github.com/AndriGitDev/meil/internal/ingest"
)

func TestExplainAlignmentPass(t *testing.T) {
	t.Parallel()

	explanation := ExplainAlignment(ingest.AggregateRecord{
		HeaderFrom:        "example.com",
		DKIMAlignedResult: "fail",
		SPFAlignedResult:  "pass",
	}, nil)
	if explanation.Result != "aligned" {
		t.Fatalf("Result = %q, want aligned", explanation.Result)
	}
}

func TestExplainAlignmentFailure(t *testing.T) {
	t.Parallel()

	explanation := ExplainAlignment(ingest.AggregateRecord{
		HeaderFrom:        "example.com",
		DKIMAlignedResult: "fail",
		SPFAlignedResult:  "fail",
	}, []ingest.AuthResult{
		{Kind: "dkim", Domain: "mailer.example.net"},
		{Kind: "spf", Domain: "bounce.example.net"},
	})
	if explanation.Result != "failed" || !strings.Contains(explanation.Detail, "mailer.example.net") || !strings.Contains(explanation.Detail, "bounce.example.net") {
		t.Fatalf("unexpected explanation: %+v", explanation)
	}
}
