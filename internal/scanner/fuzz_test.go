package scanner

import "testing"

func FuzzNormalizeDomain(f *testing.F) {
	for _, seed := range []string{
		"example.com",
		"https://Example.com/path",
		"bücher.example",
		"bad..example",
		"-bad.example",
		"localhost",
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, input string) {
		_, _ = NormalizeDomain(input)
	})
}

func FuzzParseSPF(f *testing.F) {
	for _, seed := range []string{
		"v=spf1 include:_spf.example.com -all",
		"v=spf1 redirect=_spf.example.com",
		"v=spf1 ptr ?all",
		"v=spf1 include: -all",
		"not-spf",
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, record string) {
		_ = parseSPF(record)
	})
}

func FuzzParseTagRecord(f *testing.F) {
	for _, seed := range []string{
		"v=DMARC1; p=reject; rua=mailto:dmarc@example.com",
		"v=DMARC1; p=none; pct=50; adkim=s; aspf=r",
		"v=BIMI1; l=https://example.com/logo.svg; a=https://example.com/vmc.pem",
		"v=DMARC1; p",
		"not-a-tag-record",
	} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, record string) {
		_ = parseTagRecord(record, "dmarc")
		_ = parseTagRecord(record, "bimi")
	})
}
