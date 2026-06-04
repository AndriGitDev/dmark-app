package dnsprovider

import (
	"strconv"
	"strings"
)

type Status string

const (
	StatusDetected  Status = "detected"
	StatusAmbiguous Status = "ambiguous"
	StatusUnknown   Status = "unknown"
)

type Provider struct {
	ID           string
	Name         string
	NSPatterns   []string
	DashboardURL string
}

type Detection struct {
	Status              Status
	ProviderID          string
	ProviderName        string
	Confidence          string
	EvidenceNameservers []string
	DNSDashboardURL     string
}

type RecordKind string

const (
	RecordDMARCTXT      RecordKind = "dmarc_txt"
	RecordSPFTXT        RecordKind = "spf_txt"
	RecordSPFRouteCNAME RecordKind = "spf_route_cname"
)

type RecordInstructionInput struct {
	Kind  RecordKind
	Name  string
	Value string
	TTL   int
}

type Instructions struct {
	ProviderID   string
	ProviderName string
	DashboardURL string
	Steps        []string
}

var registry = []Provider{
	{
		ID:           "isnic",
		Name:         "ISNIC",
		NSPatterns:   []string{"isnic.is"},
		DashboardURL: "https://www.isnic.is/en/domain/console",
	},
	{
		ID:           "1984",
		Name:         "1984 Hosting",
		NSPatterns:   []string{"1984.is"},
		DashboardURL: "https://control.1984.hosting/",
	},
	{
		ID:           "opin-kerfi",
		Name:         "Opin Kerfi",
		NSPatterns:   nil, // TODO: add conservative nameserver patterns when confirmed with production evidence.
		DashboardURL: "https://www.okhysing.is/",
	},
	{
		ID:           "origo",
		Name:         "Origo",
		NSPatterns:   nil, // TODO: add conservative nameserver patterns when confirmed with production evidence.
		DashboardURL: "https://www.origo.is/",
	},
	{
		ID:           "advania",
		Name:         "Advania",
		NSPatterns:   nil, // TODO: add conservative nameserver patterns when confirmed with production evidence.
		DashboardURL: "https://www.advania.is/",
	},
	{
		ID:           "cloudflare",
		Name:         "Cloudflare",
		NSPatterns:   []string{".ns.cloudflare.com", "cloudflare.com"},
		DashboardURL: "https://dash.cloudflare.com/",
	},
	{
		ID:           "route53",
		Name:         "Amazon Route 53",
		NSPatterns:   []string{"awsdns"},
		DashboardURL: "https://console.aws.amazon.com/route53/",
	},
	{
		ID:           "bunny",
		Name:         "bunny.net DNS",
		NSPatterns:   []string{"bunnydns.com", "bunny.net"},
		DashboardURL: "https://dash.bunny.net/dns",
	},
	{
		ID:           "azure-dns",
		Name:         "Microsoft Azure DNS",
		NSPatterns:   []string{"azure-dns.com", "azure-dns.net", "azure-dns.org", "azure-dns.info"},
		DashboardURL: "https://portal.azure.com/#view/HubsExtension/BrowseResource/resourceType/Microsoft.Network%2Fdnszones",
	},
	{
		ID:           "google-domains",
		Name:         "Google Domains legacy DNS",
		NSPatterns:   []string{"googledomains.com"},
		DashboardURL: "https://domains.google.com/",
	},
}

// Providers returns the static DNS provider registry used by Detect.
func Providers() []Provider {
	out := make([]Provider, len(registry))
	for i, provider := range registry {
		out[i] = provider
		if provider.NSPatterns != nil {
			out[i].NSPatterns = append([]string(nil), provider.NSPatterns...)
		}
	}
	return out
}

// Detect attempts to identify the authoritative DNS provider from nameserver hostnames.
func Detect(nameservers []string) Detection {
	evidence := normalizeNameservers(nameservers)
	matched := make(map[string]Provider)

	for _, ns := range evidence {
		for _, provider := range registry {
			if providerMatches(provider, ns) {
				matched[provider.ID] = provider
			}
		}
	}

	switch len(matched) {
	case 0:
		return Detection{
			Status:              StatusUnknown,
			Confidence:          "low",
			EvidenceNameservers: evidence,
		}
	case 1:
		for _, provider := range matched {
			return Detection{
				Status:              StatusDetected,
				ProviderID:          provider.ID,
				ProviderName:        provider.Name,
				Confidence:          "high",
				EvidenceNameservers: evidence,
				DNSDashboardURL:     provider.DashboardURL,
			}
		}
	}

	return Detection{
		Status:              StatusAmbiguous,
		Confidence:          "ambiguous",
		EvidenceNameservers: evidence,
	}
}

func normalizeNameservers(nameservers []string) []string {
	evidence := make([]string, 0, len(nameservers))
	for _, ns := range nameservers {
		normalized := strings.TrimSuffix(strings.ToLower(strings.TrimSpace(ns)), ".")
		if normalized == "" {
			continue
		}
		evidence = append(evidence, normalized)
	}
	return evidence
}

func providerMatches(provider Provider, nameserver string) bool {
	for _, pattern := range provider.NSPatterns {
		pattern = strings.TrimSuffix(strings.ToLower(strings.TrimSpace(pattern)), ".")
		if pattern == "" {
			continue
		}
		if strings.HasPrefix(pattern, ".") {
			if strings.HasSuffix(nameserver, pattern) {
				return true
			}
			continue
		}
		if pattern == "awsdns" && isRoute53Nameserver(nameserver) {
			return true
		}
		if nameserver == pattern || strings.HasSuffix(nameserver, "."+pattern) {
			return true
		}
	}
	return false
}

func isRoute53Nameserver(nameserver string) bool {
	if !strings.HasPrefix(nameserver, "ns-") || !strings.Contains(nameserver, ".awsdns-") {
		return false
	}
	for _, suffix := range []string{".com", ".net", ".org", ".co.uk"} {
		if strings.HasSuffix(nameserver, suffix) {
			return true
		}
	}
	return false
}

// BuildInstructions renders operator-facing DNS setup instructions for a record.
func BuildInstructions(d Detection, input RecordInstructionInput) Instructions {
	recordType, label := recordTypeAndLabel(input.Kind)
	providerName := d.ProviderName
	if providerName == "" {
		providerName = "Generic DNS"
	}

	instructions := Instructions{
		ProviderID:   d.ProviderID,
		ProviderName: providerName,
		DashboardURL: d.DNSDashboardURL,
		Steps:        genericSteps(providerName, recordType, label, input),
	}

	if d.Status == StatusDetected && d.ProviderID == "cloudflare" {
		instructions.Steps = cloudflareSteps(recordType, label, input)
	}

	return instructions
}

func recordTypeAndLabel(kind RecordKind) (recordType string, label string) {
	switch kind {
	case RecordDMARCTXT:
		return "TXT", "DMARC policy"
	case RecordSPFTXT:
		return "TXT", "SPF policy"
	case RecordSPFRouteCNAME:
		return "CNAME", "SPF route"
	default:
		return "TXT", "DNS"
	}
}

func genericSteps(providerName, recordType, label string, input RecordInstructionInput) []string {
	return []string{
		"Open your DNS dashboard" + providerSuffix(providerName) + " and choose the authoritative zone for the domain.",
		"Create a " + recordType + " record for the " + label + ": name " + input.Name + ", value " + input.Value + ".",
		"Set the TTL to " + ttlText(input.TTL) + ".",
		"Save the record, then wait for DNS propagation before verifying it in Dmark.",
	}
}

func cloudflareSteps(recordType, label string, input RecordInstructionInput) []string {
	return []string{
		"Open Cloudflare, select the domain, then go to DNS > Records.",
		"Add a new " + recordType + " record for the " + label + ": name " + input.Name + ", target/value " + input.Value + ".",
		"Set TTL to " + ttlText(input.TTL) + "; for CNAME records used only for SPF routing, keep the proxy disabled (DNS only).",
		"Save the record and verify it in Dmark after Cloudflare publishes the change.",
	}
}

func providerSuffix(providerName string) string {
	if providerName == "" || providerName == "Generic DNS" {
		return ""
	}
	return " for " + providerName
}

func ttlText(ttl int) string {
	if ttl <= 0 {
		return "your provider's default"
	}
	return strconv.Itoa(ttl)
}
