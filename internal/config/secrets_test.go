package config

import "testing"

func TestSecretStatuses(t *testing.T) {
	t.Parallel()

	statuses := SecretStatuses(Config{LettermintToken: "token"})
	if len(statuses) == 0 {
		t.Fatalf("expected secret statuses")
	}
	found := false
	for _, status := range statuses {
		if status.Name == "LETTERMINT_TOKEN" {
			found = true
			if !status.Present {
				t.Fatalf("LETTERMINT_TOKEN should be present: %+v", status)
			}
		}
	}
	if !found {
		t.Fatalf("LETTERMINT_TOKEN status not found: %+v", statuses)
	}
}

func TestRedactSecret(t *testing.T) {
	t.Parallel()

	if got := RedactSecret(""); got != "" {
		t.Fatalf("RedactSecret empty = %q, want empty", got)
	}
	if got := RedactSecret("short"); got != "********" {
		t.Fatalf("RedactSecret short = %q, want mask", got)
	}
	if got := RedactSecret("abcdefghijklmnopqrstuvwxyz"); got != "abcd...wxyz" {
		t.Fatalf("RedactSecret long = %q, want prefix/suffix redaction", got)
	}
}
