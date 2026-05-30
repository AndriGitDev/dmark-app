package spfflatten

import (
	"fmt"
	"net/netip"
	"sort"
	"strings"
)

const defaultGeneratedMaxLength = 450

type GenerateOptions struct {
	Terminal  string
	MaxLength int
}

type Generated struct {
	SPF     string
	Blocked bool
	Error   string
}

func Generate(mechanisms []Mechanism, opts GenerateOptions) Generated {
	terminal := strings.TrimSpace(opts.Terminal)
	if terminal == "" {
		terminal = "~all"
	}
	if !supportedTerminal(terminal) {
		return Generated{Blocked: true, Error: fmt.Sprintf("unsupported terminal %q", terminal)}
	}

	maxLength := opts.MaxLength
	if maxLength == 0 {
		maxLength = defaultGeneratedMaxLength
	}

	ip4s, err := dedupeMechanisms(mechanisms, SourceIP4)
	if err != nil {
		return Generated{Blocked: true, Error: err.Error()}
	}
	ip6s, err := dedupeMechanisms(mechanisms, SourceIP6)
	if err != nil {
		return Generated{Blocked: true, Error: err.Error()}
	}
	sort.Strings(ip4s)
	sort.Strings(ip6s)

	parts := []string{"v=spf1"}
	for _, value := range ip4s {
		parts = append(parts, "ip4:"+value)
	}
	for _, value := range ip6s {
		parts = append(parts, "ip6:"+value)
	}
	parts = append(parts, terminal)

	spf := strings.Join(parts, " ")
	if len(spf) > maxLength {
		return Generated{Blocked: true, Error: fmt.Sprintf("generated SPF length %d exceeds max length %d", len(spf), maxLength)}
	}

	return Generated{SPF: spf}
}

func supportedTerminal(terminal string) bool {
	switch terminal {
	case "~all", "-all", "?all":
		return true
	default:
		return false
	}
}

func dedupeMechanisms(mechanisms []Mechanism, kind SourceKind) ([]string, error) {
	seen := make(map[string]struct{})
	var values []string
	for _, mechanism := range mechanisms {
		if mechanism.Kind != kind {
			continue
		}
		value := strings.TrimSpace(mechanism.Value)
		if value == "" {
			continue
		}
		if err := validateIPMechanismValue(kind, value); err != nil {
			return nil, err
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		values = append(values, value)
	}
	return values, nil
}

func validateIPMechanismValue(kind SourceKind, value string) error {
	if len(strings.Fields(value)) != 1 {
		return fmt.Errorf("invalid %s mechanism value %q", kind, value)
	}

	addrText := value
	if strings.Contains(value, "/") {
		prefix, err := netip.ParsePrefix(value)
		if err != nil {
			return fmt.Errorf("invalid %s mechanism value %q", kind, value)
		}
		addrText = prefix.Addr().String()
	}

	addr, err := netip.ParseAddr(addrText)
	if err != nil {
		return fmt.Errorf("invalid %s mechanism value %q", kind, value)
	}
	if kind == SourceIP4 && !addr.Is4() {
		return fmt.Errorf("invalid ip4 mechanism value %q", value)
	}
	if kind == SourceIP6 && !addr.Is6() {
		return fmt.Errorf("invalid ip6 mechanism value %q", value)
	}
	return nil
}
