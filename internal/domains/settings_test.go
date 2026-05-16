package domains

import (
	"testing"
	"time"
)

func TestNormalizeSettings(t *testing.T) {
	t.Parallel()

	settings, err := NormalizeSettings(Settings{
		NotificationEmail: " Alerts@Example.COM ",
		DKIMSelectors:     []string{"Google", "google", "selector1"},
		ExpectedSenders:   []string{" Mailchimp ", "mailchimp"},
		IgnoredFindings:   []string{"bimi_missing", ""},
	})
	if err != nil {
		t.Fatalf("NormalizeSettings returned error: %v", err)
	}
	if settings.NotificationEmail != "alerts@example.com" || settings.ScanCadence != 24*time.Hour {
		t.Fatalf("unexpected normalized settings: %+v", settings)
	}
	if len(settings.DKIMSelectors) != 2 || settings.DKIMSelectors[0] != "google" {
		t.Fatalf("selectors = %#v, want normalized unique selectors", settings.DKIMSelectors)
	}
	if len(settings.ExpectedSenders) != 1 || settings.ExpectedSenders[0] != "mailchimp" {
		t.Fatalf("expected senders = %#v, want normalized unique senders", settings.ExpectedSenders)
	}
}

func TestNormalizeSettingsRejectsInvalidValues(t *testing.T) {
	t.Parallel()

	if _, err := NormalizeSettings(Settings{NotificationEmail: "not an email"}); err == nil {
		t.Fatalf("NormalizeSettings returned nil error for invalid email")
	}
	if _, err := NormalizeSettings(Settings{ScanCadence: time.Minute}); err == nil {
		t.Fatalf("NormalizeSettings returned nil error for too-frequent cadence")
	}
}
