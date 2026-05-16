package abuse

import "testing"

func TestEvaluate(t *testing.T) {
	t.Parallel()

	policy := DefaultPolicy()
	tests := []struct {
		signal Signal
		want   Verdict
	}{
		{signal: Signal{UserAgent: "Mozilla/5.0", RequestsInWindow: 1}, want: VerdictAllow},
		{signal: Signal{UserAgent: "Mozilla/5.0", RequestsInWindow: policy.ChallengeAfterRequests}, want: VerdictChallenge},
		{signal: Signal{UserAgent: "Mozilla/5.0", RequestsInWindow: policy.BlockAfterRequests}, want: VerdictBlock},
		{signal: Signal{UserAgent: "Mozilla/5.0", FailuresInWindow: policy.BlockAfterFailures}, want: VerdictBlock},
		{signal: Signal{UserAgent: "sqlmap"}, want: VerdictBlock},
	}
	for _, tt := range tests {
		if got := Evaluate(tt.signal, policy); got != tt.want {
			t.Fatalf("Evaluate(%+v) = %q, want %q", tt.signal, got, tt.want)
		}
	}
}
