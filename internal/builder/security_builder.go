package builder

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/idlab-discover/aibomgen-cli/internal/fetcher"

	cdx "github.com/CycloneDX/cyclonedx-go"
)

// securityScannerLabel maps internal scanner field names to human-readable labels.
var securityScannerLabel = map[string]string{
	"avScan":           "Cisco Foundation AI (ClamAV)",
	"protectAiScan":    "ProtectAI",
	"pickleImportScan": "HuggingFace Pickle Scanner",
	"virusTotalScan":   "VirusTotal",
	"jFrogScan":        "JFrog Research",
}

// statusToSeverity maps a scanner status string to a CycloneDX severity.
func statusToSeverity(status string) cdx.Severity {
	switch strings.ToLower(status) {
	case "unsafe":
		return cdx.SeverityCritical
	case "suspicious":
		return cdx.SeverityHigh
	case "caution":
		return cdx.SeverityMedium
	default:
		return cdx.SeverityUnknown
	}
}

// isActionable returns true when a scanner status warrants a vulnerability rating.
// (i.e. excludes "safe" and "unscanned").
func isActionable(status string) bool {
	s := strings.ToLower(status)
	return s != "" && s != "safe" && s != "unscanned"
}

// InjectSecurityData appends BOM.Vulnerabilities derived from the HF tree.
// security scan results. Summary Component.Properties are handled by the.
// metadata registry (fields_security.go) which runs before this call.
// revision is the scanned model revision (empty means main) and is used in file links.
// It is a no-op when entries is empty.
func InjectSecurityData(bom *cdx.BOM, comp *cdx.Component, entries []fetcher.SecurityFileEntry, modelID, revision string) {
	if len(entries) == 0 {
		return
	}

	var vulns []cdx.Vulnerability
	for _, entry := range entries {
		if entry.SecurityFileStatus == nil || !isActionable(entry.SecurityFileStatus.Status) {
			continue
		}
		vulns = append(vulns, buildFileVulnerability(entry, comp, modelID, revision))
	}

	if len(vulns) == 0 {
		return
	}

	if bom.Vulnerabilities == nil {
		bom.Vulnerabilities = &[]cdx.Vulnerability{}
	}
	*bom.Vulnerabilities = append(*bom.Vulnerabilities, vulns...)
}

// buildFileVulnerability converts a single SecurityFileEntry into a CycloneDX.
// Vulnerability, aggregating findings from all per-file scanners.
func buildFileVulnerability(entry fetcher.SecurityFileEntry, comp *cdx.Component, modelID, revision string) cdx.Vulnerability {
	sfs := entry.SecurityFileStatus

	// Collect per-scanner ratings.
	type scannerEntry struct {
		key    string
		status string
		msg    string
		link   string
		label  string
	}
	scanners := []scannerEntry{
		{"avScan", sfs.AvScan.Status, sfs.AvScan.Message, sfs.AvScan.ReportLink, sfs.AvScan.ReportLabel},
		{"protectAiScan", sfs.ProtectAiScan.Status, sfs.ProtectAiScan.Message, sfs.ProtectAiScan.ReportLink, sfs.ProtectAiScan.ReportLabel},
		{"pickleImportScan", sfs.PickleImportScan.Status, sfs.PickleImportScan.Message, "", ""},
		{"virusTotalScan", sfs.VirusTotalScan.Status, sfs.VirusTotalScan.Message, sfs.VirusTotalScan.ReportLink, sfs.VirusTotalScan.ReportLabel},
		{"jFrogScan", sfs.JFrogScan.Status, sfs.JFrogScan.Message, sfs.JFrogScan.ReportLink, sfs.JFrogScan.ReportLabel},
	}

	var ratings []cdx.VulnerabilityRating
	var advisories []cdx.Advisory
	var descParts []string

	for _, sc := range scanners {
		if !isActionable(sc.status) {
			continue
		}
		label := securityScannerLabel[sc.key]
		sev := statusToSeverity(sc.status)

		ratings = append(ratings, cdx.VulnerabilityRating{
			Source:   &cdx.Source{Name: label},
			Severity: sev,
			Method:   cdx.ScoringMethodOther,
		})

		if sc.msg != "" {
			descParts = append(descParts, fmt.Sprintf("[%s] %s", label, sc.msg))
		}
		if sc.link != "" {
			advisoryTitle := sc.label
			if advisoryTitle == "" {
				advisoryTitle = label + " report"
			}
			advisories = append(advisories, cdx.Advisory{
				Title: advisoryTitle,
				URL:   sc.link,
			})
		}
	}

	// No scanner detail: rate from the file's overall status so the finding keeps a severity.
	if len(ratings) == 0 {
		ratings = append(ratings, cdx.VulnerabilityRating{Severity: statusToSeverity(sfs.Status), Method: cdx.ScoringMethodOther})
	}

	// Include pickle import details in description.
	if isActionable(sfs.PickleImportScan.Status) && len(sfs.PickleImportScan.PickleImports) > 0 {
		var imports []string
		for _, pi := range sfs.PickleImportScan.PickleImports {
			imports = append(imports, fmt.Sprintf("%s.%s (%s)", pi.Module, pi.Name, pi.Safety))
		}
		descParts = append(descParts, "Pickle imports: "+strings.Join(imports, ", "))
	}

	// Build description.
	description := fmt.Sprintf("Security finding in file %q (overall status: %s)", entry.Path, sfs.Status)
	if len(descParts) > 0 {
		description += ". " + strings.Join(descParts, "; ")
	}

	// Sanitise path for use in BOMRef.
	safePath := strings.ReplaceAll(entry.Path, "/", "-")
	safePath = strings.ReplaceAll(safePath, " ", "_")

	vuln := cdx.Vulnerability{
		BOMRef: fmt.Sprintf("hfsec-%s-%s", comp.BOMRef, safePath),
		Source: &cdx.Source{
			Name: "HuggingFace Security Scanner",
			URL:  fmt.Sprintf("https://huggingface.co/%s/blob/%s/%s", modelID, blobRevision(revision), entry.Path),
		},
		Description: description,
		Affects: &[]cdx.Affects{
			{Ref: comp.BOMRef},
		},
	}
	if len(ratings) > 0 {
		vuln.Ratings = &ratings
	}
	if len(advisories) > 0 {
		vuln.Advisories = &advisories
	}

	return vuln
}

// blobRevision returns the path-escaped revision for file links, defaulting to main.
func blobRevision(revision string) string {
	if rev := strings.TrimSpace(revision); rev != "" {
		return url.PathEscape(rev)
	}
	return "main"
}
