package monitoring

import (
	"context"

	"github.com/AndriGitDev/meil/internal/alerts"
	"github.com/AndriGitDev/meil/internal/drift"
	"github.com/AndriGitDev/meil/internal/scanner"
)

type Scanner interface {
	Scan(ctx context.Context, domain string) (scanner.Result, error)
}

type ScanInput struct {
	Domain    string
	Previous  scanner.Result
	AlertRule alerts.Rule
	Recipient string
}

type ScanOutput struct {
	Current scanner.Result
	Events  []drift.Event
	Emails  []alerts.EmailPayload
}

func RunScan(ctx context.Context, emailScanner Scanner, input ScanInput) (ScanOutput, error) {
	current, err := emailScanner.Scan(ctx, input.Domain)
	if err != nil {
		return ScanOutput{}, err
	}
	events := drift.Compare(input.Previous, current)
	output := ScanOutput{Current: current, Events: events}
	for _, event := range events {
		decision := alerts.ShouldNotify(event, input.AlertRule, current.CheckedAt)
		if decision.Send {
			output.Emails = append(output.Emails, alerts.BuildEmailPayload(current.Domain, event))
		}
	}
	return output, nil
}
