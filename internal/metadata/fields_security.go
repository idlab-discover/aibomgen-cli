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
			return overallSecurityStatus(src.SecurityTree), true
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

// overallSecurityStatus derives a summary status string from the full tree.
// Returns "unsafe", "caution", or "safe".
func overallSecurityStatus(entries []fetcher.SecurityFileEntry) string {
	hasUnsafe := false
	hasCaution := false
	for _, e := range entries {
		if e.SecurityFileStatus == nil {
			continue
		}
		switch strings.ToLower(e.SecurityFileStatus.Status) {
		case "unsafe":
			hasUnsafe = true
		case "caution":
			hasCaution = true
		}
	}
	if hasUnsafe {
		return "unsafe"
	}
	if hasCaution {
		return "caution"
	}
	return "safe"
}
