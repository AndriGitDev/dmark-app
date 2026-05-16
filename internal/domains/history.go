package domains

import "github.com/AndriGitDev/meil/internal/scanner"

type CheckType string

const (
	CheckTypeSPF   CheckType = "spf"
	CheckTypeDKIM  CheckType = "dkim"
	CheckTypeDMARC CheckType = "dmarc"
	CheckTypeBIMI  CheckType = "bimi"
)

type HistoryFilter struct {
	Status scanner.Status
	Type   CheckType
}

type HistoryEntry struct {
	DomainID string
	Domain   string
	Check    Check
}

func FilterHistory(entries []HistoryEntry, filter HistoryFilter) []HistoryEntry {
	if filter.Status == "" && filter.Type == "" {
		copied := make([]HistoryEntry, len(entries))
		copy(copied, entries)
		return copied
	}
	filtered := []HistoryEntry{}
	for _, entry := range entries {
		if filter.Type != "" {
			check := checkByType(entry.Check, filter.Type)
			if check.Name == "" {
				continue
			}
			if filter.Status != "" && check.Status != filter.Status {
				continue
			}
			filtered = append(filtered, entry)
			continue
		}
		if filter.Status != "" && !entryHasStatus(entry, filter.Status) {
			continue
		}
		filtered = append(filtered, entry)
	}
	return filtered
}

func checkByType(check Check, checkType CheckType) scanner.CheckResult {
	switch checkType {
	case CheckTypeSPF:
		return check.SPF
	case CheckTypeDKIM:
		return check.DKIM
	case CheckTypeDMARC:
		return check.DMARC
	case CheckTypeBIMI:
		return check.BIMI
	default:
		return scanner.CheckResult{}
	}
}

func entryHasStatus(entry HistoryEntry, status scanner.Status) bool {
	return entry.Check.SPF.Status == status ||
		entry.Check.DKIM.Status == status ||
		entry.Check.DMARC.Status == status ||
		entry.Check.BIMI.Status == status
}
