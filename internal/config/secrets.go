package config

import "strings"

type SecretStatus struct {
	Name     string
	Present  bool
	Required bool
}

func SecretStatuses(c Config) []SecretStatus {
	return []SecretStatus{
		{Name: "LETTERMINT_TOKEN", Present: strings.TrimSpace(c.LettermintToken) != "", Required: false},
		{Name: "BILLING_WEBHOOK_SECRET", Present: strings.TrimSpace(c.BillingWebhookSecret) != "", Required: false},
	}
}

func MissingRequiredSecrets(c Config) []string {
	missing := []string{}
	for _, status := range SecretStatuses(c) {
		if status.Required && !status.Present {
			missing = append(missing, status.Name)
		}
	}
	return missing
}

func RedactSecret(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	if len(value) <= 8 {
		return "********"
	}
	return value[:4] + "..." + value[len(value)-4:]
}
