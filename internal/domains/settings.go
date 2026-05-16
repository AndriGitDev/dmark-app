package domains

import (
	"fmt"
	"net/mail"
	"strings"
	"time"
)

type Settings struct {
	NotificationEmail string
	DKIMSelectors     []string
	ExpectedSenders   []string
	ScanCadence       time.Duration
	IgnoredFindings   []string
}

func NormalizeSettings(settings Settings) (Settings, error) {
	if settings.ScanCadence == 0 {
		settings.ScanCadence = 24 * time.Hour
	}
	if settings.ScanCadence < time.Hour {
		return Settings{}, fmt.Errorf("scan cadence must be at least one hour")
	}
	if settings.NotificationEmail != "" {
		email := strings.TrimSpace(strings.ToLower(settings.NotificationEmail))
		addr, err := mail.ParseAddress(email)
		if err != nil || addr.Address != email {
			return Settings{}, fmt.Errorf("invalid notification email")
		}
		settings.NotificationEmail = email
	}
	settings.DKIMSelectors = normalizeList(settings.DKIMSelectors)
	settings.ExpectedSenders = normalizeList(settings.ExpectedSenders)
	settings.IgnoredFindings = normalizeList(settings.IgnoredFindings)
	return settings, nil
}

func normalizeList(values []string) []string {
	result := []string{}
	seen := map[string]struct{}{}
	for _, value := range values {
		value = strings.TrimSpace(strings.ToLower(value))
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
