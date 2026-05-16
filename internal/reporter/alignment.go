package reporter

import (
	"strings"

	"github.com/AndriGitDev/meil/internal/ingest"
)

type AlignmentExplanation struct {
	Result  string
	Summary string
	Detail  string
}

func ExplainAlignment(record ingest.AggregateRecord, authResults []ingest.AuthResult) AlignmentExplanation {
	if record.DKIMAlignedResult == "pass" || record.SPFAlignedResult == "pass" {
		return AlignmentExplanation{
			Result:  "aligned",
			Summary: "At least one authentication method aligned with the visible From domain.",
			Detail:  "DMARC passes when either SPF or DKIM both authenticate and align with the header-from domain.",
		}
	}
	dkimDomains := domainsFor(authResults, "dkim")
	spfDomains := domainsFor(authResults, "spf")
	detail := "Neither DKIM nor SPF aligned with " + record.HeaderFrom + "."
	if len(dkimDomains) > 0 {
		detail += " DKIM authenticated domains: " + strings.Join(dkimDomains, ", ") + "."
	}
	if len(spfDomains) > 0 {
		detail += " SPF authenticated domains: " + strings.Join(spfDomains, ", ") + "."
	}
	return AlignmentExplanation{
		Result:  "failed",
		Summary: "Mail failed DMARC alignment.",
		Detail:  detail,
	}
}

func domainsFor(results []ingest.AuthResult, kind string) []string {
	domains := []string{}
	seen := map[string]struct{}{}
	for _, result := range results {
		if result.Kind != kind || result.Domain == "" {
			continue
		}
		domain := strings.ToLower(result.Domain)
		if _, ok := seen[domain]; ok {
			continue
		}
		seen[domain] = struct{}{}
		domains = append(domains, domain)
	}
	return domains
}
