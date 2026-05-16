package attribution

type EnrichmentFixture struct {
	Name                string
	Lookup              EnrichmentLookup
	AuthenticatedDomain string
	WantProvider        string
	WantConfidence      Confidence
	WantAmbiguous       bool
}

func ProviderReviewFixtures() []EnrichmentFixture {
	return []EnrichmentFixture{
		{
			Name: "google_high_confidence",
			Lookup: EnrichmentLookup{
				IP:          "203.0.113.10",
				ReverseDNS:  "mail-sor-f41.google.com.",
				ASN:         "AS15169",
				CountryCode: "US",
			},
			AuthenticatedDomain: "_spf.google.com",
			WantProvider:        "Google Workspace",
			WantConfidence:      ConfidenceHigh,
		},
		{
			Name: "sendgrid_medium_confidence",
			Lookup: EnrichmentLookup{
				IP:          "203.0.113.20",
				ReverseDNS:  "o1.email.sendgrid.net.",
				ASN:         "AS11377",
				CountryCode: "IS",
			},
			AuthenticatedDomain: "example.com",
			WantProvider:        "SendGrid",
			WantConfidence:      ConfidenceMedium,
		},
		{
			Name: "unclassified_source",
			Lookup: EnrichmentLookup{
				IP:          "203.0.113.30",
				ReverseDNS:  "mail.unknown.example.",
				ASN:         "AS64500",
				CountryCode: "DE",
			},
			AuthenticatedDomain: "unknown.example",
			WantConfidence:      ConfidenceNone,
		},
	}
}
