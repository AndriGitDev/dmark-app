package ingest

import "testing"

func TestProvisionRUA(t *testing.T) {
	t.Parallel()

	provisioning, err := ProvisionRUA("Example.COM", RUAConfig{LocalPartPrefix: "Reports", MailboxDomain: "ingest.meil.test"})
	if err != nil {
		t.Fatalf("ProvisionRUA returned error: %v", err)
	}
	if provisioning.Domain != "example.com" {
		t.Fatalf("Domain = %q, want example.com", provisioning.Domain)
	}
	if provisioning.Address != "reports+example=com@ingest.meil.test" {
		t.Fatalf("Address = %q, want reports+example=com@ingest.meil.test", provisioning.Address)
	}
	if provisioning.DMARCTag != "rua=mailto:reports+example=com@ingest.meil.test" || len(provisioning.Instructions) == 0 {
		t.Fatalf("unexpected provisioning: %+v", provisioning)
	}
}

func TestProvisionRUARejectsInvalidMailboxDomain(t *testing.T) {
	t.Parallel()

	if _, err := ProvisionRUA("example.com", RUAConfig{MailboxDomain: "localhost"}); err == nil {
		t.Fatalf("ProvisionRUA returned nil error for invalid mailbox domain")
	}
}
