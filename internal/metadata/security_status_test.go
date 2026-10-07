package metadata

import (
	"testing"

	"github.com/idlab-discover/aibomgen-cli/internal/fetcher"
)

func TestOverallSecurityStatus(t *testing.T) {
	entries := func(statuses ...string) []fetcher.SecurityFileEntry {
		var out []fetcher.SecurityFileEntry
		for _, s := range statuses {
			out = append(out, fetcher.SecurityFileEntry{SecurityFileStatus: &fetcher.SecurityFileStatus{Status: s}})
		}
		return append(out, fetcher.SecurityFileEntry{}) // entry without scan data is ignored
	}
	for want, in := range map[string][]fetcher.SecurityFileEntry{
		"safe":       entries("safe", "unscanned", "queued"),
		"caution":    entries("safe", "caution"),
		"suspicious": entries("caution", "Suspicious", "safe"),
		"unsafe":     entries("suspicious", "unsafe", "caution"),
	} {
		if got := OverallSecurityStatus(in); got != want {
			t.Errorf("OverallSecurityStatus = %q, want %q", got, want)
		}
	}
}
