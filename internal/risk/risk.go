package risk

import (
	"github.com/AndriGitDev/meil/internal/drift"
	"github.com/AndriGitDev/meil/internal/reporter"
	"github.com/AndriGitDev/meil/internal/scanner"
)

type Inputs struct {
	Posture      scanner.PostureSummary
	DriftEvents  []drift.Event
	Senders      []reporter.SenderInventoryItem
	ReportHealth reporter.HealthResult
}

type Score struct {
	Value   int
	Level   string
	Reasons []string
}

func Calculate(inputs Inputs) Score {
	value := 100 - inputs.Posture.Score
	reasons := []string{}
	if inputs.Posture.Status != scanner.StatusPass {
		reasons = append(reasons, inputs.Posture.TopIssue)
	}
	for _, event := range inputs.DriftEvents {
		switch event.Severity {
		case drift.SeverityCritical:
			value += 30
			reasons = append(reasons, event.Message)
		case drift.SeverityWarn:
			value += 15
			reasons = append(reasons, event.Message)
		}
	}
	for _, sender := range inputs.Senders {
		switch reporter.InventoryRisk(sender) {
		case "high":
			value += 30
			reasons = append(reasons, "high-risk sender detected")
		case "review":
			value += 15
			reasons = append(reasons, "sender needs review")
		}
	}
	switch inputs.ReportHealth.Status {
	case "ingestion_failures", "missing_reports":
		value += 20
		reasons = append(reasons, inputs.ReportHealth.Message)
	case "no_reports":
		value += 10
		reasons = append(reasons, inputs.ReportHealth.Message)
	}
	if value > 100 {
		value = 100
	}
	return Score{Value: value, Level: level(value), Reasons: dedupe(reasons)}
}

func level(value int) string {
	switch {
	case value >= 75:
		return "critical"
	case value >= 45:
		return "high"
	case value >= 20:
		return "medium"
	default:
		return "low"
	}
}

func dedupe(values []string) []string {
	result := []string{}
	seen := map[string]struct{}{}
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}
