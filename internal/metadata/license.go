package metadata

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"sync"

	cdx "github.com/CycloneDX/cyclonedx-go"
)

// spdxSchemaJSON is a verbatim copy of schema/spdx.schema.json from
// github.com/CycloneDX/cyclonedx-go v0.12.0: the SPDX license ID enum that the
// CycloneDX 1.6/1.7 JSON schemas validate license.id against.
// Refresh it when bumping cyclonedx-go.
//
//go:embed spdx.schema.json
var spdxSchemaJSON []byte

var (
	spdxOnce sync.Once
	spdxIDs  map[string]string // lowercase ID -> canonical ID
)

func loadSPDX() {
	var schema struct {
		Enum []string `json:"enum"`
	}
	if err := json.Unmarshal(spdxSchemaJSON, &schema); err != nil {
		panic(fmt.Sprintf("invalid embedded spdx.schema.json: %v", err))
	}
	spdxIDs = make(map[string]string, len(schema.Enum))
	for _, id := range schema.Enum {
		spdxIDs[strings.ToLower(id)] = id
	}
}

// spdxID returns the canonical SPDX license ID matching v case-insensitively.
func spdxID(v string) (string, bool) {
	spdxOnce.Do(loadSPDX)
	id, ok := spdxIDs[strings.ToLower(strings.TrimSpace(v))]
	return id, ok
}

// licenseInput is the raw license information gathered from Hugging Face sources.
type licenseInput struct {
	Values []string // license identifiers as written on HF (may include "other" and placeholders)
	Name   string   // license_name (used when the value is "other")
	Link   string   // license_link (absolute URL or path relative to the repo root)
	// RepoPath ("org/model" or "datasets/org/ds") and SHA locate relative license links
	// in the repo at the resolved commit; relative links are dropped when either is unknown.
	RepoPath string
	SHA      string
	// LicenseFile is a license file at the repository root (e.g. "LICENSE"), if any.
	LicenseFile string
}

// licenseFileRe matches a license file name at the repository root.
var licenseFileRe = regexp.MustCompile(`(?i)^licen[cs]e(\.(txt|md|rst))?$`)

// rootLicenseFile returns the license file at the repository root, preferring "LICENSE".
func rootLicenseFile(files []string) string {
	found := ""
	for _, f := range files {
		if strings.Contains(f, "/") || !licenseFileRe.MatchString(f) {
			continue
		}
		if f == "LICENSE" {
			return f
		}
		if found == "" {
			found = f
		}
	}
	return found
}

// spdxLicenseURL is the SPDX license list page for an SPDX license ID.
func spdxLicenseURL(id string) string {
	return "https://spdx.org/licenses/" + id + ".html"
}

// firstRealGroup returns the first group that still has values after dropping placeholders.
func firstRealGroup(groups ...[]string) []string {
	for _, g := range groups {
		var out []string
		for _, v := range g {
			if !isPlaceholder(v) {
				out = append(out, strings.TrimSpace(v))
			}
		}
		if len(out) > 0 {
			return out
		}
	}
	return nil
}

func licenseTags(tags []string) []string {
	var out []string
	for _, t := range tags {
		t = strings.TrimSpace(t)
		if strings.HasPrefix(t, "license:") {
			out = append(out, strings.TrimSpace(strings.TrimPrefix(t, "license:")))
		}
	}
	return out
}

func blobBase(baseURL, repoPath, sha string) string {
	repoPath = strings.Trim(strings.TrimSpace(repoPath), "/")
	sha = strings.ToLower(strings.TrimSpace(sha))
	if repoPath == "" || sha == "" {
		return ""
	}
	return hfBaseURL(baseURL) + repoPath + "/blob/" + sha + "/"
}

// modelLicenseInput gathers license data for a model.
// Order: cardData.license, API license, license:* tags, README front matter license.
func modelLicenseInput(src Source) (licenseInput, bool) {
	var cardData, front map[string]any
	var groups [][]string
	var tags []string
	var in licenseInput
	if src.HF != nil {
		cardData = src.HF.CardData
		groups = append(groups, stringsFromAny(cardData["license"]), stringsFromAny(src.HF.License))
		tags = src.HF.Tags
		id := strings.TrimSpace(src.HF.ID)
		if id == "" {
			id = strings.TrimSpace(src.ModelID)
		}
		in.RepoPath, in.SHA = id, src.HF.SHA
		files := make([]string, 0, len(src.HF.Siblings))
		for _, s := range src.HF.Siblings {
			files = append(files, s.RFilename)
		}
		in.LicenseFile = rootLicenseFile(files)
	}
	groups = append(groups, licenseTags(tags))
	if src.Readme != nil {
		front = src.Readme.FrontMatter
		groups = append(groups, stringsFromAny(front["license"]))
	}
	in.Values = firstRealGroup(groups...)
	if len(in.Values) == 0 {
		return licenseInput{}, false
	}
	in.Name = realString(firstString("license_name", cardData, front))
	in.Link = realString(firstString("license_link", cardData, front))
	return in, true
}

// datasetLicenseInput gathers license data for a dataset.
// Order: cardData.license, license:* tags, README front matter license.
func datasetLicenseInput(src DatasetSource) (licenseInput, bool) {
	var cardData, front map[string]any
	var groups [][]string
	var in licenseInput
	if src.HF != nil {
		cardData = src.HF.CardData
		groups = append(groups, stringsFromAny(cardData["license"]), licenseTags(src.HF.Tags))
		id := strings.TrimSpace(src.HF.ID)
		if id == "" {
			id = strings.TrimSpace(src.DatasetID)
		}
		if id != "" {
			in.RepoPath, in.SHA = "datasets/"+id, src.HF.SHA
		}
	}
	if src.Readme != nil {
		front = src.Readme.FrontMatter
		groups = append(groups, stringsFromAny(front["license"]))
	}
	in.Values = firstRealGroup(groups...)
	if len(in.Values) == 0 {
		return licenseInput{}, false
	}
	in.Name = realString(firstString("license_name", cardData, front))
	in.Link = realString(firstString("license_link", cardData, front))
	return in, true
}

// licenseURL resolves license_link to an absolute URL; relative links need a known commit.
func licenseURL(in licenseInput, baseURL string) string {
	link := strings.TrimSpace(in.Link)
	if link == "" {
		return ""
	}
	lower := strings.ToLower(link)
	if strings.HasPrefix(lower, "https://") || strings.HasPrefix(lower, "http://") {
		return link
	}
	base := blobBase(baseURL, in.RepoPath, in.SHA)
	if strings.Contains(link, "://") || base == "" {
		return ""
	}
	link = strings.TrimLeft(strings.TrimPrefix(link, "./"), "/")
	if link == "" {
		return ""
	}
	return base + link
}

// buildLicenses converts HF license data into CycloneDX licenses.
// SPDX IDs (case-insensitive) become license.id in canonical casing, anything else license.name.
// Placeholders are dropped; nil means no license is known and the field must be omitted.
// license.url: for a single license, license_link, else the repository's license file,
// else the SPDX page; with several licenses, only the SPDX pages (a single link or file
// can't be attributed to one of them).
func buildLicenses(in licenseInput, baseURL string) *cdx.Licenses {
	var ls cdx.Licenses
	seen := map[string]struct{}{}
	for _, v := range in.Values {
		v = realString(v)
		if v == "" {
			continue
		}
		if strings.EqualFold(v, "other") && in.Name != "" {
			v = in.Name
		}
		lic := &cdx.License{}
		if id, ok := spdxID(v); ok {
			lic.ID = id
		} else {
			lic.Name = v
		}
		key := strings.ToLower(lic.ID + "|" + lic.Name)
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}
		ls = append(ls, cdx.LicenseChoice{License: lic})
	}
	if len(ls) == 0 {
		return nil
	}
	if len(ls) == 1 {
		ls[0].License.URL = licenseURL(in, baseURL)
		if ls[0].License.URL == "" && in.LicenseFile != "" {
			if base := blobBase(baseURL, in.RepoPath, in.SHA); base != "" {
				ls[0].License.URL = base + in.LicenseFile
			}
		}
	}
	for i := range ls {
		if l := ls[i].License; l.URL == "" && l.ID != "" {
			l.URL = spdxLicenseURL(l.ID)
		}
	}
	return &ls
}

// licenseInputFromValue accepts a gathered licenseInput or a plain user-provided string.
func licenseInputFromValue(value any) (licenseInput, bool) {
	switch v := value.(type) {
	case licenseInput:
		return v, true
	case string:
		if s := strings.TrimSpace(v); s != "" {
			return licenseInput{Values: []string{s}}, true
		}
	}
	return licenseInput{}, false
}
