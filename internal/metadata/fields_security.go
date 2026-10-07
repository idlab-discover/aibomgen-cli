package metadata

import (
	"strconv"
	"strings"

	"github.com/idlab-discover/aibomgen-cli/internal/fetcher"
)

func securityFields() []FieldSpec {
	return []FieldSpec{
		hfProp(ComponentPropertiesSecurityOverallStatus, 0.3, func(src Source) (any, bool) {
			if len(src.SecurityTree) == 0 {
				return nil, false
			}
			return OverallSecurityStatus(src.SecurityTree), true
		}),
		hfProp(ComponentPropertiesSecurityScannedFiles, 0.2, func(src Source) (any, bool) {
			if len(src.SecurityTree) == 0 {
				return nil, false
			}
			return countStatus(src.SecurityTree, ""), true
		}),
		hfProp(ComponentPropertiesSecurityUnsafeFiles, 0.2, func(src Source) (any, bool) {
			if len(src.SecurityTree) == 0 {
				return nil, false
			}
			return countStatus(src.SecurityTree, "unsafe"), true
		}),
		hfProp(ComponentPropertiesSecurityCautionFiles, 0.2, func(src Source) (any, bool) {
			if len(src.SecurityTree) == 0 {
				return nil, false
			}
			return countStatus(src.SecurityTree, "caution"), true
		}),
	}
}

// countStatus counts scanned files with the given status ("" counts every scanned file).
func countStatus(entries []fetcher.SecurityFileEntry, status string) string {
	n := 0
	for _, e := range entries {
		if e.SecurityFileStatus != nil && (status == "" || strings.ToLower(e.SecurityFileStatus.Status) == status) {
			n++
		}
	}
	return strconv.Itoa(n)
}

// OverallSecurityStatus derives a summary status from the full security tree:
// the worst file status of "unsafe", "suspicious" or "caution", else "safe".
func OverallSecurityStatus(entries []fetcher.SecurityFileEntry) string {
	worst := "safe"
	rank := map[string]int{"safe": 0, "caution": 1, "suspicious": 2, "unsafe": 3}
	for _, e := range entries {
		if e.SecurityFileStatus == nil {
			continue
		}
		if status := strings.ToLower(e.SecurityFileStatus.Status); rank[status] > rank[worst] {
			worst = status
		}
	}
	return worst
}
