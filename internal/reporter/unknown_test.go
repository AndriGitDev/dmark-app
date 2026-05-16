package reporter

import "testing"

func TestDetectUnknownSenders(t *testing.T) {
	t.Parallel()

	alerts := DetectUnknownSenders([]SenderInventoryItem{
		{Provider: "Known", Expected: true, Volume: 100},
		{Provider: "", Volume: 50, FailureRate: 0.02},
		{Provider: "Suspicious ESP", Suspicious: true, Volume: 25},
		{Provider: "Tiny", Volume: 1, FailureRate: 1},
		{Provider: "Ignored", Ignored: true, Volume: 100, FailureRate: 1},
	}, UnknownSenderRule{MinimumVolume: 10})

	if len(alerts) != 2 {
		t.Fatalf("len(alerts) = %d, want 2: %#v", len(alerts), alerts)
	}
	if alerts[0].Volume != 50 || alerts[0].Reason != "sender provider is unclassified" {
		t.Fatalf("unexpected first alert: %+v", alerts[0])
	}
	if alerts[1].Reason != "sender is marked suspicious" {
		t.Fatalf("unexpected second alert: %+v", alerts[1])
	}
}

func TestDetectUnknownSendersRespectsFailureThreshold(t *testing.T) {
	t.Parallel()

	alerts := DetectUnknownSenders([]SenderInventoryItem{
		{Provider: "Known", Volume: 100, FailureRate: 0.20},
		{Provider: "Known low fail", Volume: 100, FailureRate: 0.01},
	}, UnknownSenderRule{MinimumVolume: 10, MinimumFailureRate: 0.10})

	if len(alerts) != 1 || alerts[0].Provider != "Known" {
		t.Fatalf("alerts = %#v, want only high-failure sender", alerts)
	}
}
