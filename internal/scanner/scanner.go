package scanner

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"
)

type Status string

const (
	StatusPass    Status = "pass"
	StatusWarn    Status = "warn"
	StatusFail    Status = "fail"
	StatusMissing Status = "missing"
	StatusUnknown Status = "unknown"
)

type Resolver interface {
	LookupTXT(ctx context.Context, name string) ([]string, error)
}

type netResolver struct{}

func (netResolver) LookupTXT(ctx context.Context, name string) ([]string, error) {
	return net.DefaultResolver.LookupTXT(ctx, name)
}

type Scanner struct {
	resolver  Resolver
	timeout   time.Duration
	selectors []string
}

type Result struct {
	Domain string
	SPF    CheckResult
	DMARC  CheckResult
	BIMI   CheckResult
	DKIM   CheckResult
}

type CheckResult struct {
	Name      string
	Status    Status
	Message   string
	Record    string
	Selectors []SelectorResult
}

type SelectorResult struct {
	Selector string
	Record   string
	Status   Status
}

var commonDKIMSelectors = []string{
	"default", "dkim", "dkim1", "dkim2", "mail", "key1", "key2",
	"google", "selector1", "selector2", "s1", "s2",
	"protonmail", "protonmail2", "protonmail3", "fm1", "fm2", "fm3", "sig1", "tutanota1", "tutanota2", "zoho", "zmail", "hey", "s2048", "s1024",
	"k1", "k2", "k3", "mandrill", "mailchimp", "mailo", "mta", "pic", "smtp", "pm", "20150623", "ck", "convertkit", "dk", "litmus", "everlytickey1", "everlytickey2", "sm", "scph",
}

func New(resolver Resolver) *Scanner {
	if resolver == nil {
		resolver = netResolver{}
	}
	return &Scanner{
		resolver:  resolver,
		timeout:   3 * time.Second,
		selectors: commonDKIMSelectors,
	}
}

func CommonDKIMSelectors() []string {
	selectors := make([]string, len(commonDKIMSelectors))
	copy(selectors, commonDKIMSelectors)
	return selectors
}

func NormalizeDomain(input string) (string, error) {
	domain := strings.TrimSpace(strings.ToLower(input))
	if domain == "" {
		return "", errors.New("domain is required")
	}

	if strings.Contains(domain, "://") {
		parsed, err := url.Parse(domain)
		if err != nil {
			return "", fmt.Errorf("parsing domain URL: %w", err)
		}
		domain = parsed.Hostname()
	}

	domain = strings.TrimSuffix(domain, ".")
	if strings.Contains(domain, "/") || strings.Contains(domain, " ") || strings.Contains(domain, "..") {
		return "", fmt.Errorf("invalid domain %q", input)
	}

	labels := strings.Split(domain, ".")
	if len(labels) < 2 {
		return "", fmt.Errorf("invalid domain %q", input)
	}
	for _, label := range labels {
		if label == "" || strings.HasPrefix(label, "-") || strings.HasSuffix(label, "-") || len(label) > 63 {
			return "", fmt.Errorf("invalid domain %q", input)
		}
		for _, r := range label {
			if (r < 'a' || r > 'z') && (r < '0' || r > '9') && r != '-' {
				return "", fmt.Errorf("invalid domain %q", input)
			}
		}
	}

	return domain, nil
}

func (s *Scanner) Scan(ctx context.Context, input string) (Result, error) {
	domain, err := NormalizeDomain(input)
	if err != nil {
		return Result{}, err
	}

	return Result{
		Domain: domain,
		SPF:    s.scanSPF(ctx, domain),
		DMARC:  s.scanDMARC(ctx, domain),
		BIMI:   s.scanBIMI(ctx, domain),
		DKIM:   s.scanDKIM(ctx, domain),
	}, nil
}

func (s *Scanner) scanSPF(ctx context.Context, domain string) CheckResult {
	records, err := s.lookupTXT(ctx, domain)
	if err != nil {
		return lookupErrorResult("SPF", err)
	}

	spfRecords := matchingRecords(records, "v=spf1")
	if len(spfRecords) == 0 {
		return CheckResult{Name: "SPF", Status: StatusMissing, Message: "No SPF record found on the root domain."}
	}
	if len(spfRecords) > 1 {
		return CheckResult{Name: "SPF", Status: StatusFail, Message: "Multiple SPF records are published; receivers treat this as a permerror.", Record: strings.Join(spfRecords, "\n")}
	}

	record := spfRecords[0]
	if strings.Contains(strings.ToLower(record), " -all") {
		return CheckResult{Name: "SPF", Status: StatusPass, Message: "SPF is published with a hard-fail policy.", Record: record}
	}
	return CheckResult{Name: "SPF", Status: StatusWarn, Message: "SPF is published but does not end in hard-fail (-all).", Record: record}
}

func (s *Scanner) scanDMARC(ctx context.Context, domain string) CheckResult {
	records, err := s.lookupTXT(ctx, "_dmarc."+domain)
	if err != nil {
		return lookupErrorResult("DMARC", err)
	}

	dmarcRecords := matchingRecords(records, "v=dmarc1")
	if len(dmarcRecords) == 0 {
		return CheckResult{Name: "DMARC", Status: StatusMissing, Message: "No DMARC record found."}
	}
	if len(dmarcRecords) > 1 {
		return CheckResult{Name: "DMARC", Status: StatusFail, Message: "Multiple DMARC records are published; receivers treat this as invalid.", Record: strings.Join(dmarcRecords, "\n")}
	}

	record := dmarcRecords[0]
	lower := strings.ToLower(record)
	if !strings.Contains(lower, "p=") {
		return CheckResult{Name: "DMARC", Status: StatusFail, Message: "DMARC record is missing the required p= policy.", Record: record}
	}
	if strings.Contains(lower, "p=reject") || strings.Contains(lower, "p=quarantine") {
		return CheckResult{Name: "DMARC", Status: StatusPass, Message: "DMARC is enforcing mail authentication failures.", Record: record}
	}
	return CheckResult{Name: "DMARC", Status: StatusWarn, Message: "DMARC is in monitoring mode (p=none), so spoofed mail is not blocked.", Record: record}
}

func (s *Scanner) scanBIMI(ctx context.Context, domain string) CheckResult {
	records, err := s.lookupTXT(ctx, "default._bimi."+domain)
	if err != nil {
		return lookupErrorResult("BIMI", err)
	}

	bimiRecords := matchingRecords(records, "v=bimi1")
	if len(bimiRecords) == 0 {
		return CheckResult{Name: "BIMI", Status: StatusMissing, Message: "No BIMI record found at default._bimi." + domain + "."}
	}
	return CheckResult{Name: "BIMI", Status: StatusPass, Message: "BIMI record is published.", Record: bimiRecords[0]}
}

func (s *Scanner) scanDKIM(ctx context.Context, domain string) CheckResult {
	selectors := make([]SelectorResult, 0, len(s.selectors))
	unknown := false

	for _, selector := range s.selectors {
		name := selector + "._domainkey." + domain
		records, err := s.lookupTXT(ctx, name)
		if err != nil {
			if isNotFound(err) {
				selectors = append(selectors, SelectorResult{Selector: selector, Status: StatusMissing})
				continue
			}
			unknown = true
			selectors = append(selectors, SelectorResult{Selector: selector, Status: StatusUnknown})
			continue
		}

		record := firstMatchingRecord(records, "v=dkim1")
		if record == "" {
			selectors = append(selectors, SelectorResult{Selector: selector, Status: StatusWarn, Record: strings.Join(records, "\n")})
			continue
		}
		if hasDKIMKey(record) {
			selectors = append(selectors, SelectorResult{Selector: selector, Status: StatusPass, Record: record})
			return CheckResult{Name: "DKIM", Status: StatusPass, Message: "DKIM key found under selector " + selector + ".", Record: record, Selectors: selectors}
		}
		selectors = append(selectors, SelectorResult{Selector: selector, Status: StatusWarn, Record: record})
	}

	if unknown {
		return CheckResult{Name: "DKIM", Status: StatusUnknown, Message: "Some DKIM selector lookups failed; DKIM status could not be confirmed.", Selectors: selectors}
	}
	return CheckResult{Name: "DKIM", Status: StatusMissing, Message: fmt.Sprintf("No DKIM record found across %d common selectors. The domain may use a custom selector.", len(s.selectors)), Selectors: selectors}
}

func (s *Scanner) lookupTXT(ctx context.Context, name string) ([]string, error) {
	lookupCtx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()
	return s.resolver.LookupTXT(lookupCtx, name)
}

func lookupErrorResult(name string, err error) CheckResult {
	if isNotFound(err) {
		return CheckResult{Name: name, Status: StatusMissing, Message: name + " record was not found."}
	}
	return CheckResult{Name: name, Status: StatusUnknown, Message: "DNS lookup failed: " + err.Error()}
}

func isNotFound(err error) bool {
	var dnsErr *net.DNSError
	return errors.As(err, &dnsErr) && dnsErr.IsNotFound
}

func matchingRecords(records []string, prefix string) []string {
	matches := []string{}
	for _, record := range records {
		if strings.HasPrefix(strings.ToLower(strings.TrimSpace(record)), prefix) {
			matches = append(matches, record)
		}
	}
	return matches
}

func firstMatchingRecord(records []string, prefix string) string {
	matches := matchingRecords(records, prefix)
	if len(matches) == 0 {
		return ""
	}
	return matches[0]
}

func hasDKIMKey(record string) bool {
	if !strings.Contains(strings.ToLower(record), "v=dkim1") {
		return false
	}
	parts := strings.Split(record, ";")
	for _, part := range parts {
		key, value, ok := strings.Cut(strings.TrimSpace(part), "=")
		if ok && strings.EqualFold(strings.TrimSpace(key), "p") {
			return strings.TrimSpace(value) != ""
		}
	}
	return false
}
