package ingest

import (
	"fmt"
	"strings"

	"github.com/AndriGitDev/meil/internal/scanner"
)

type RUAConfig struct {
	LocalPartPrefix string
	MailboxDomain   string
}

type RUAProvisioning struct {
	Domain        string
	Address       string
	DMARCTag      string
	ForwardTarget string
	Instructions  []string
}

func ProvisionRUA(domain string, config RUAConfig) (RUAProvisioning, error) {
	normalized, err := scanner.NormalizeDomain(domain)
	if err != nil {
		return RUAProvisioning{}, err
	}
	mailboxDomain, err := scanner.NormalizeDomain(config.MailboxDomain)
	if err != nil {
		return RUAProvisioning{}, fmt.Errorf("invalid RUA mailbox domain: %w", err)
	}
	prefix := strings.Trim(strings.ToLower(config.LocalPartPrefix), " .-_")
	if prefix == "" {
		prefix = "dmarc"
	}
	localPart := prefix + "+" + strings.ReplaceAll(normalized, ".", "=")
	address := localPart + "@" + mailboxDomain
	return RUAProvisioning{
		Domain:        normalized,
		Address:       address,
		DMARCTag:      "rua=mailto:" + address,
		ForwardTarget: address,
		Instructions: []string{
			"add or update the domain DMARC record with " + "rua=mailto:" + address,
			"keep the existing p, sp, adkim, and aspf tags unchanged unless intentionally changing policy",
			"forward aggregate report attachments to " + address + " if the DNS host or mailbox provider uses a separate reporting mailbox",
		},
	}, nil
}
