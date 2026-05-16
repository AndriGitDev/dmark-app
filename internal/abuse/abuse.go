package abuse

import "strings"

type Verdict string

const (
	VerdictAllow     Verdict = "allow"
	VerdictChallenge Verdict = "challenge"
	VerdictBlock     Verdict = "block"
)

type Signal struct {
	Identifier       string
	RequestsInWindow int
	FailuresInWindow int
	UserAgent        string
}

type Policy struct {
	ChallengeAfterRequests int
	BlockAfterRequests     int
	BlockAfterFailures     int
}

func DefaultPolicy() Policy {
	return Policy{ChallengeAfterRequests: 20, BlockAfterRequests: 60, BlockAfterFailures: 10}
}

func Evaluate(signal Signal, policy Policy) Verdict {
	if policy.ChallengeAfterRequests <= 0 {
		policy = DefaultPolicy()
	}
	userAgent := strings.ToLower(strings.TrimSpace(signal.UserAgent))
	if userAgent == "" || strings.Contains(userAgent, "sqlmap") || strings.Contains(userAgent, "nikto") {
		return VerdictBlock
	}
	if signal.FailuresInWindow >= policy.BlockAfterFailures || signal.RequestsInWindow >= policy.BlockAfterRequests {
		return VerdictBlock
	}
	if signal.RequestsInWindow >= policy.ChallengeAfterRequests {
		return VerdictChallenge
	}
	return VerdictAllow
}
