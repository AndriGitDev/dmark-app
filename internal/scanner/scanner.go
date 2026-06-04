package scanner

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/idna"
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
	dkimLimit int
}

type Result struct {
	Domain    string
	CheckedAt time.Time
	Duration  time.Duration
	SPF       CheckResult
	DMARC     CheckResult
	BIMI      CheckResult
	DKIM      CheckResult
}

type CheckResult struct {
	Name           string
	Status         Status
	Message        string
	Record         string
	SourceName     string
	LookupDuration time.Duration
	ErrorClass     string
	Selectors      []SelectorResult
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
		dkimLimit: 6,
	}
}

func NewWithSelectors(resolver Resolver, selectors []string) *Scanner {
	scanner := New(resolver)
	if len(selectors) > 0 {
		scanner.selectors = normalizeSelectors(selectors)
	}
	return scanner
}

func CommonDKIMSelectors() []string {
	selectors := make([]string, len(commonDKIMSelectors))
	copy(selectors, commonDKIMSelectors)
	return selectors
}

func ProviderDKIMSelectors(provider string) []string {
	switch strings.ToLower(strings.TrimSpace(provider)) {
	case "google", "google-workspace":
		return []string{"google", "selector1", "selector2"}
	case "microsoft", "microsoft-365", "office365":
		return []string{"selector1", "selector2"}
	case "proton", "protonmail":
		return []string{"protonmail", "protonmail2", "protonmail3"}
	case "fastmail":
		return []string{"fm1", "fm2", "fm3"}
	case "zoho":
		return []string{"zoho", "zmail"}
	default:
		return nil
	}
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
	if net.ParseIP(domain) != nil {
		return "", fmt.Errorf("invalid domain %q", input)
	}
	if strings.Contains(domain, "/") || strings.Contains(domain, " ") || strings.Contains(domain, "..") {
		return "", fmt.Errorf("invalid domain %q", input)
	}
	asciiDomain, err := idna.Lookup.ToASCII(domain)
	if err != nil {
		return "", fmt.Errorf("invalid domain %q", input)
	}
	domain = strings.ToLower(asciiDomain)
	if net.ParseIP(domain) != nil {
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
	checkedAt := time.Now().UTC()
	started := time.Now()

	spf := s.scanSPF(ctx, domain)
	dmarc := s.scanDMARC(ctx, domain)
	bimi := s.scanBIMI(ctx, domain, dmarc)
	dkim := s.scanDKIM(ctx, domain)
	return Result{
		Domain:    domain,
		CheckedAt: checkedAt,
		Duration:  time.Since(started),
		SPF:       spf,
		DMARC:     dmarc,
		BIMI:      bimi,
		DKIM:      dkim,
	}, nil
}

func (s *Scanner) scanSPF(ctx context.Context, domain string) CheckResult {
	lookup := s.lookupTXT(ctx, domain)
	if lookup.err != nil {
		return lookupErrorResult("SPF", lookup)
	}

	spfRecords := matchingRecords(lookup.records, "v=spf1")
	if len(spfRecords) == 0 {
		return checkResult("SPF", StatusMissing, "No SPF record found on the root domain.", "", lookup)
	}
	if len(spfRecords) > 1 {
		return checkResult("SPF", StatusFail, "Multiple SPF records are published; receivers treat this as a permerror.", strings.Join(spfRecords, "\n"), lookup)
	}

	record := spfRecords[0]
	parsed := parseSPF(record)
	if len(parsed.errors) > 0 {
		return checkResult("SPF", StatusFail, "SPF record has syntax errors: "+strings.Join(parsed.errors, "; ")+".", record, lookup)
	}
	if parsed.lookupCount > 10 {
		return checkResult("SPF", StatusFail, fmt.Sprintf("SPF uses %d DNS-querying mechanisms; receivers allow at most 10.", parsed.lookupCount), record, lookup)
	}
	if parsed.hasPTR {
		return checkResult("SPF", StatusWarn, "SPF uses the deprecated ptr mechanism; replace it with explicit ip4/ip6/include mechanisms.", record, lookup)
	}
	if parsed.allQualifier == "+" {
		return checkResult("SPF", StatusFail, "SPF ends with +all, authorizing every sender on the internet.", record, lookup)
	}
	if parsed.allQualifier == "-" {
		return checkResult("SPF", StatusPass, "SPF is published with a hard-fail policy.", record, lookup)
	}
	if parsed.allQualifier == "~" {
		return checkResult("SPF", StatusWarn, "SPF is published with soft-fail (~all); unauthorized senders may still be accepted.", record, lookup)
	}
	if parsed.allQualifier == "?" {
		return checkResult("SPF", StatusWarn, "SPF ends with neutral (?all), so it does not clearly reject unauthorized senders.", record, lookup)
	}
	if parsed.hasRedirect {
		return checkResult("SPF", StatusPass, "SPF delegates policy with redirect; delegated policy should be monitored too.", record, lookup)
	}
	return checkResult("SPF", StatusWarn, "SPF is published but does not contain an all mechanism or redirect.", record, lookup)
}

func (s *Scanner) scanDMARC(ctx context.Context, domain string) CheckResult {
	lookup := s.lookupTXT(ctx, "_dmarc."+domain)
	if lookup.err != nil {
		return lookupErrorResult("DMARC", lookup)
	}

	dmarcRecords := matchingRecords(lookup.records, "v=dmarc1")
	if len(dmarcRecords) == 0 {
		return checkResult("DMARC", StatusMissing, "No DMARC record found.", "", lookup)
	}
	if len(dmarcRecords) > 1 {
		return checkResult("DMARC", StatusFail, "Multiple DMARC records are published; receivers treat this as invalid.", strings.Join(dmarcRecords, "\n"), lookup)
	}

	record := dmarcRecords[0]
	parsed := parseTagRecord(record, "dmarc")
	if len(parsed.errors) > 0 {
		return checkResult("DMARC", StatusFail, "DMARC record has syntax errors: "+strings.Join(parsed.errors, "; ")+".", record, lookup)
	}
	policy := strings.ToLower(parsed.tags["p"])
	if policy == "" {
		return checkResult("DMARC", StatusFail, "DMARC record is missing the required p= policy.", record, lookup)
	}
	if policy != "none" && policy != "quarantine" && policy != "reject" {
		return checkResult("DMARC", StatusFail, "DMARC p= policy must be none, quarantine, or reject.", record, lookup)
	}
	if err := validateDMARCOptionalTags(parsed.tags); err != nil {
		return checkResult("DMARC", StatusFail, err.Error(), record, lookup)
	}
	if policy == "none" {
		return checkResult("DMARC", StatusWarn, "DMARC is in monitoring mode (p=none), so spoofed mail is not blocked.", record, lookup)
	}
	if strings.EqualFold(parsed.tags["sp"], "none") {
		return checkResult("DMARC", StatusWarn, "DMARC enforces the root domain but leaves subdomains in monitoring mode with sp=none.", record, lookup)
	}
	if pct := parsed.tags["pct"]; pct != "" && pct != "100" {
		return checkResult("DMARC", StatusWarn, "DMARC enforcement applies to only "+pct+"% of failing mail.", record, lookup)
	}
	return checkResult("DMARC", StatusPass, "DMARC is enforcing mail authentication failures.", record, lookup)
}

func (s *Scanner) scanBIMI(ctx context.Context, domain string, dmarc CheckResult) CheckResult {
	lookup := s.lookupTXT(ctx, "default._bimi."+domain)
	if lookup.err != nil {
		return lookupErrorResult("BIMI", lookup)
	}

	bimiRecords := matchingRecords(lookup.records, "v=bimi1")
	if len(bimiRecords) == 0 {
		return checkResult("BIMI", StatusMissing, "No BIMI record found at default._bimi."+domain+".", "", lookup)
	}
	if len(bimiRecords) > 1 {
		return checkResult("BIMI", StatusFail, "Multiple BIMI records are published; receivers expect one default BIMI assertion.", strings.Join(bimiRecords, "\n"), lookup)
	}
	record := bimiRecords[0]
	parsed := parseTagRecord(record, "bimi")
	if len(parsed.errors) > 0 {
		return checkResult("BIMI", StatusFail, "BIMI record has syntax errors: "+strings.Join(parsed.errors, "; ")+".", record, lookup)
	}
	logo := parsed.tags["l"]
	if logo == "" {
		return checkResult("BIMI", StatusFail, "BIMI record is missing the required l= SVG logo URI.", record, lookup)
	}
	logoURL, err := url.Parse(logo)
	if err != nil || logoURL.Scheme != "https" || logoURL.Host == "" || !strings.HasSuffix(strings.ToLower(logoURL.Path), ".svg") {
		return checkResult("BIMI", StatusFail, "BIMI l= must be an HTTPS URI that points to an SVG logo.", record, lookup)
	}
	if dmarc.Status != StatusPass {
		return checkResult("BIMI", StatusWarn, "BIMI is published, but DMARC must be enforced with quarantine or reject for broad mailbox support.", record, lookup)
	}
	if cert := parsed.tags["a"]; cert == "" {
		return checkResult("BIMI", StatusWarn, "BIMI logo is published without a VMC/CMC certificate URI in a=.", record, lookup)
	} else if certURL, err := url.Parse(cert); err != nil || certURL.Scheme != "https" || certURL.Host == "" {
		return checkResult("BIMI", StatusFail, "BIMI a= certificate URI must use HTTPS.", record, lookup)
	}
	return checkResult("BIMI", StatusPass, "BIMI record is published with an SVG logo and certificate URI.", record, lookup)
}

func (s *Scanner) scanDKIM(ctx context.Context, domain string) CheckResult {
	started := time.Now()
	selectors := make([]SelectorResult, len(s.selectors))
	dkimLimit := s.dkimLimit
	if dkimLimit < 1 {
		dkimLimit = 1
	}
	sem := make(chan struct{}, dkimLimit)
	var wg sync.WaitGroup
	for i, selector := range s.selectors {
		wg.Add(1)
		go func(i int, selector string) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			selectors[i] = s.scanDKIMSelector(ctx, domain, selector)
		}(i, selector)
	}
	wg.Wait()

	unknown := false
	for i, selector := range selectors {
		if selector.Status == StatusPass {
			return CheckResult{Name: "DKIM", Status: StatusPass, Message: "DKIM key found under selector " + selector.Selector + ".", Record: selector.Record, SourceName: selector.Selector + "._domainkey." + domain, LookupDuration: time.Since(started), Selectors: selectors[:i+1]}
		}
		if selector.Status == StatusUnknown {
			unknown = true
		}
	}
	if unknown {
		return CheckResult{Name: "DKIM", Status: StatusUnknown, Message: "Some DKIM selector lookups failed; DKIM status could not be confirmed.", LookupDuration: time.Since(started), ErrorClass: "resolver_failure", Selectors: selectors}
	}
	return CheckResult{Name: "DKIM", Status: StatusMissing, Message: fmt.Sprintf("No DKIM record found across %d common selectors. The domain may use a custom selector.", len(s.selectors)), LookupDuration: time.Since(started), Selectors: selectors}
}

func (s *Scanner) scanDKIMSelector(ctx context.Context, domain string, selector string) SelectorResult {
	name := selector + "._domainkey." + domain
	lookup := s.lookupTXT(ctx, name)
	if lookup.err != nil {
		if isNotFound(lookup.err) {
			return SelectorResult{Selector: selector, Status: StatusMissing}
		}
		return SelectorResult{Selector: selector, Status: StatusUnknown}
	}

	record := firstMatchingRecord(lookup.records, "v=dkim1")
	if record == "" {
		return SelectorResult{Selector: selector, Status: StatusWarn, Record: strings.Join(lookup.records, "\n")}
	}
	if hasDKIMKey(record) {
		return SelectorResult{Selector: selector, Status: StatusPass, Record: record}
	}
	return SelectorResult{Selector: selector, Status: StatusWarn, Record: record}
}

type lookupResult struct {
	source   string
	records  []string
	err      error
	duration time.Duration
}

func (s *Scanner) lookupTXT(ctx context.Context, name string) lookupResult {
	started := time.Now()
	lookupCtx, cancel := context.WithTimeout(ctx, s.timeout)
	defer cancel()
	records, err := s.resolver.LookupTXT(lookupCtx, name)
	return lookupResult{source: name, records: records, err: err, duration: time.Since(started)}
}

func lookupErrorResult(name string, lookup lookupResult) CheckResult {
	errorClass := classifyLookupError(lookup.err)
	if errorClass == "not_found" {
		return CheckResult{Name: name, Status: StatusMissing, Message: name + " record was not found.", SourceName: lookup.source, LookupDuration: lookup.duration, ErrorClass: errorClass}
	}
	return CheckResult{Name: name, Status: StatusUnknown, Message: "DNS lookup failed: " + lookup.err.Error(), SourceName: lookup.source, LookupDuration: lookup.duration, ErrorClass: errorClass}
}

func checkResult(name string, status Status, message string, record string, lookup lookupResult) CheckResult {
	return CheckResult{Name: name, Status: status, Message: message, Record: record, SourceName: lookup.source, LookupDuration: lookup.duration}
}

func isNotFound(err error) bool {
	var dnsErr *net.DNSError
	return errors.As(err, &dnsErr) && dnsErr.IsNotFound
}

func classifyLookupError(err error) string {
	if err == nil {
		return ""
	}
	if isNotFound(err) {
		return "not_found"
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return "timeout"
	}
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) && dnsErr.IsTimeout {
		return "timeout"
	}
	return "resolver_failure"
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

func normalizeSelectors(selectors []string) []string {
	normalized := []string{}
	seen := map[string]struct{}{}
	for _, selector := range selectors {
		selector = strings.TrimSpace(strings.ToLower(selector))
		if selector == "" {
			continue
		}
		if strings.Contains(selector, "._domainkey.") {
			selector, _, _ = strings.Cut(selector, "._domainkey.")
		}
		if _, ok := seen[selector]; ok {
			continue
		}
		seen[selector] = struct{}{}
		normalized = append(normalized, selector)
	}
	return normalized
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

type spfParseResult struct {
	allQualifier string
	hasPTR       bool
	hasRedirect  bool
	lookupCount  int
	errors       []string
}

func parseSPF(record string) spfParseResult {
	var result spfParseResult
	fields := strings.Fields(record)
	if len(fields) == 0 || !strings.EqualFold(fields[0], "v=spf1") {
		result.errors = append(result.errors, "missing v=spf1 version")
		return result
	}
	seenAll := false
	for _, field := range fields[1:] {
		if strings.Contains(field, "=") {
			key, value, _ := strings.Cut(field, "=")
			key = strings.ToLower(strings.TrimSpace(key))
			if value == "" {
				result.errors = append(result.errors, key+" modifier has no value")
				continue
			}
			switch key {
			case "redirect":
				result.lookupCount++
				result.hasRedirect = true
			case "exp":
				result.lookupCount++
			default:
				result.errors = append(result.errors, "unknown modifier "+key)
			}
			continue
		}

		qualifier := "+"
		mechanism := field
		if strings.ContainsAny(field[:1], "+-~?") {
			qualifier = field[:1]
			mechanism = field[1:]
		}
		name := mechanism
		if before, _, ok := strings.Cut(name, ":"); ok {
			name = before
		}
		if before, _, ok := strings.Cut(name, "/"); ok {
			name = before
		}
		name = strings.ToLower(name)
		switch name {
		case "all":
			seenAll = true
			result.allQualifier = qualifier
		case "include", "a", "mx", "ptr", "exists":
			result.lookupCount++
			if name == "ptr" {
				result.hasPTR = true
			}
		case "ip4", "ip6":
			if !strings.Contains(mechanism, ":") {
				result.errors = append(result.errors, name+" mechanism requires a value")
			}
		default:
			result.errors = append(result.errors, "unknown mechanism "+name)
		}
	}
	if result.hasRedirect && seenAll {
		result.errors = append(result.errors, "redirect modifier must not be used with all mechanism")
	}
	return result
}

type tagParseResult struct {
	tags   map[string]string
	errors []string
}

func parseTagRecord(record string, recordType string) tagParseResult {
	result := tagParseResult{tags: map[string]string{}}
	parts := strings.Split(record, ";")
	for _, raw := range parts {
		part := strings.TrimSpace(raw)
		if part == "" {
			continue
		}
		key, value, ok := strings.Cut(part, "=")
		if !ok {
			result.errors = append(result.errors, "malformed tag "+part)
			continue
		}
		key = strings.ToLower(strings.TrimSpace(key))
		value = strings.TrimSpace(value)
		if key == "" || value == "" {
			result.errors = append(result.errors, "empty "+recordType+" tag")
			continue
		}
		if _, exists := result.tags[key]; exists {
			result.errors = append(result.errors, "duplicate tag "+key)
			continue
		}
		result.tags[key] = value
	}
	return result
}

func validateDMARCOptionalTags(tags map[string]string) error {
	for _, tag := range []string{"sp"} {
		value := strings.ToLower(tags[tag])
		if value != "" && value != "none" && value != "quarantine" && value != "reject" {
			return fmt.Errorf("DMARC %s= policy must be none, quarantine, or reject", tag)
		}
	}
	for _, tag := range []string{"adkim", "aspf"} {
		value := strings.ToLower(tags[tag])
		if value != "" && value != "r" && value != "s" {
			return fmt.Errorf("DMARC %s= alignment must be r or s", tag)
		}
	}
	if pct := tags["pct"]; pct != "" {
		value, err := strconv.Atoi(pct)
		if err != nil || value < 0 || value > 100 {
			return fmt.Errorf("DMARC pct= must be an integer from 0 to 100")
		}
	}
	for _, tag := range []string{"rua", "ruf"} {
		if value := tags[tag]; value != "" {
			for _, rawURI := range strings.Split(value, ",") {
				uri := strings.TrimSpace(rawURI)
				parsed, err := url.Parse(uri)
				if err != nil || parsed.Scheme != "mailto" || parsed.Opaque == "" {
					return fmt.Errorf("DMARC %s= contains invalid reporting URI %q", tag, uri)
				}
			}
		}
	}
	return nil
}
