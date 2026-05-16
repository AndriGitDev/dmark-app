package statuspage

import (
	"testing"
	"time"
)

func TestOverallStatus(t *testing.T) {
	t.Parallel()

	if got := OverallStatus([]Component{{Name: "checker", Status: StatusOperational}}); got != StatusOperational {
		t.Fatalf("OverallStatus operational = %q", got)
	}
	if got := OverallStatus([]Component{{Name: "checker", Status: StatusOperational}, {Name: "worker", Status: StatusDegraded}}); got != StatusDegraded {
		t.Fatalf("OverallStatus degraded = %q", got)
	}
	if got := OverallStatus([]Component{{Name: "checker", Status: StatusDegraded}, {Name: "reporter", Status: StatusOutage}}); got != StatusOutage {
		t.Fatalf("OverallStatus outage = %q", got)
	}
}

func TestIncidentIsResolved(t *testing.T) {
	t.Parallel()

	if (Incident{}).IsResolved() {
		t.Fatalf("zero incident should not be resolved")
	}
	if !(Incident{ResolvedAt: time.Now()}).IsResolved() {
		t.Fatalf("incident with resolved time should be resolved")
	}
}
