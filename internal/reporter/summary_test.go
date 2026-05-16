package reporter

import (
	"net"
	"testing"

	"github.com/AndriGitDev/meil/internal/ingest"
)

func TestSummarizeSources(t *testing.T) {
	t.Parallel()

	summaries := SummarizeSources([]ingest.AggregateRecord{
		{SourceIP: net.ParseIP("203.0.113.10"), Count: 10, Disposition: "none", DKIMAlignedResult: "pass", SPFAlignedResult: "fail", HeaderFrom: "example.com"},
		{SourceIP: net.ParseIP("203.0.113.10"), Count: 5, Disposition: "reject", DKIMAlignedResult: "fail", SPFAlignedResult: "fail", HeaderFrom: "example.com"},
		{SourceIP: net.ParseIP("203.0.113.20"), Count: 50, Disposition: "none", DKIMAlignedResult: "pass", SPFAlignedResult: "pass", HeaderFrom: "example.com"},
	}, "Mailbox")

	if len(summaries) != 2 {
		t.Fatalf("len(summaries) = %d, want 2", len(summaries))
	}
	if summaries[0].SourceIP.String() != "203.0.113.20" || summaries[0].TotalCount != 50 {
		t.Fatalf("summaries not sorted by volume: %+v", summaries)
	}
	if summaries[1].PassCount != 10 || summaries[1].FailCount != 5 || summaries[1].Disposition["reject"] != 5 || summaries[1].Organizations["Mailbox"] != 15 {
		t.Fatalf("unexpected merged summary: %+v", summaries[1])
	}
}

func TestInventoryRisk(t *testing.T) {
	t.Parallel()

	tests := []struct {
		item SenderInventoryItem
		want string
	}{
		{item: SenderInventoryItem{Provider: "Google", Expected: true, FailureRate: 0.01}, want: "expected"},
		{item: SenderInventoryItem{Provider: "Unknown", Suspicious: true}, want: "high"},
		{item: SenderInventoryItem{FailureRate: 0.20}, want: "review"},
		{item: SenderInventoryItem{Provider: "Known", FailureRate: 0.01}, want: "low"},
	}
	for _, tt := range tests {
		if got := InventoryRisk(tt.item); got != tt.want {
			t.Fatalf("InventoryRisk(%+v) = %q, want %q", tt.item, got, tt.want)
		}
	}
}
