package domains

import (
	"strings"
	"testing"
)

func TestNewOwnershipChallenge(t *testing.T) {
	t.Parallel()

	challenge, err := NewOwnershipChallenge("example.com")
	if err != nil {
		t.Fatalf("NewOwnershipChallenge returned error: %v", err)
	}
	if challenge.Name != "_meil-verify.example.com" || !strings.HasPrefix(challenge.Value, "meil-verify=") {
		t.Fatalf("unexpected challenge: %+v", challenge)
	}
}

func TestVerifyOwnership(t *testing.T) {
	t.Parallel()

	if !VerifyOwnership("meil-verify=abc", []string{"other", " meil-verify=abc "}) {
		t.Fatalf("VerifyOwnership returned false for matching TXT record")
	}
	if VerifyOwnership("meil-verify=abc", []string{"meil-verify=def"}) {
		t.Fatalf("VerifyOwnership returned true for nonmatching TXT record")
	}
}
