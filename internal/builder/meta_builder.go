package builder

import (
	"strings"
	"time"

	"github.com/CycloneDX/cyclonedx-go"
	"github.com/google/uuid"
)

// AddMetaSerialNumber sets a serial number if not already set.
func AddMetaSerialNumber(bom *cyclonedx.BOM) error {
	if bom.SerialNumber == "" {
		bom.SerialNumber = "urn:uuid:" + generateUUID()
	}
	return nil
}

// Generate UUID using google/uuid.
func generateUUID() string {
	return uuid.New().String()
}

// AddMetaTimestamp sets the timestamp if not already set.
func AddMetaTimestamp(bom *cyclonedx.BOM) error {
	if bom.Metadata.Timestamp == "" {
		bom.Metadata.Timestamp = CurrentTimestampRFC3339()
	}
	return nil
}

// CurrentTimestamp returns now formatted as RFC3339 (e.g. 2026-01-22T10:41:24+01:00).
func CurrentTimestampRFC3339() string {
	return time.Now().Format(time.RFC3339)
}

const (
	DefaultToolVendor  = "idlab-discover"
	DefaultToolName    = "aibomgen-cli"
	DefaultToolVersion = "v0.0.0"
)

// DefaultToolAuthors.
var DefaultToolAuthors = []cyclonedx.OrganizationalContact{
	{
		Name:  "Wiebe Vandendriessche",
		Email: "wiebe.vandendriessche@ugent.be",
	},
}

// AddMetaTools adds a Component entry for the tool into bom.metadata.tools.Components.
// If toolName or toolVersion are empty the defaults above are used.
func AddMetaTools(bom *cyclonedx.BOM, toolName string, toolVersion string) error {
	if bom.Metadata == nil {
		bom.Metadata = &cyclonedx.Metadata{}
	}
	if bom.Metadata.Tools == nil {
		bom.Metadata.Tools = &cyclonedx.ToolsChoice{}
	}

	name := toolName
	if name == "" {
		name = DefaultToolName
	}
	version := toolVersion
	if version == "" {
		version = DefaultToolVersion
	}

	comp := cyclonedx.Component{
		Type: cyclonedx.ComponentTypeApplication,
		Manufacturer: &cyclonedx.OrganizationalEntity{
			Name: DefaultToolVendor,
		},
		Name:    name,
		Version: version,
		Authors: &DefaultToolAuthors,
	}

	if bom.Metadata.Tools.Components == nil {
		bom.Metadata.Tools.Components = &[]cyclonedx.Component{comp}
	} else {
		components := append(*bom.Metadata.Tools.Components, comp)
		bom.Metadata.Tools.Components = &components
	}

	return nil
}

// GeneratePurl generates a package URL (purl) for a given kind, id, and version.
// URL-encode segments.
func GeneratePurl(kind string, id string, version string) string {
	// kind is model or dataset otherwise its unknown.
	if kind != "model" && kind != "dataset" {
		kind = "unknown"
	}
	if id == "" {
		id = "unknown"
	}
	var base string
	switch kind {
	case "model":
		// models use pkg:huggingface/<namespace>/<name>.
		base = "pkg:huggingface/" + id
	case "dataset":
		// datasets use plural 'datasets'.
		base = "pkg:huggingface/datasets/" + id
	default:
		base = "pkg:huggingface/" + kind + "/" + id
	}

	if version == "" {
		return base
	}
	return base + "@" + strings.ToLower(version)
}

// NormalizeSegment safe-encodes /, @, spaces, etc. in purl segments.
func NormalizeSegment(segment string) string {
	normalized := ""
	for _, ch := range segment {
		switch ch {
		case '@':
			normalized += "%40"
		case ' ':
			normalized += "%20"
		default:
			normalized += string(ch)
		}
	}
	return normalized
}

// PurlFromComponentMeta generates a purl using HF fields in meta (owner/name, lastModified, sha).
// Generated according to the purl-spec: https://github.com/package-url/purl-spec/blob/main/types-doc/huggingface-definition.md.
func PurlFromComponentMeta(kind string, id string, lastModified string, sha string) string {
	// id may be "namespace/name"; encode each segment separately so the slash remains.
	id = strings.TrimSpace(id)
	var normID string
	if id == "" {
		normID = "unknown"
	} else {
		parts := strings.Split(id, "/")
		for i, p := range parts {
			parts[i] = NormalizeSegment(strings.TrimSpace(p))
		}
		normID = strings.Join(parts, "/")
	}

	// version: prefer sha (lowercased) if present; otherwise omit.
	version := strings.ToLower(strings.TrimSpace(sha))
	return GeneratePurl(kind, normID, version)
}

// AddComponentPurl computes a deterministic pkg:huggingface purl from component metadata.
// and sets Component.PURL if not already set.
func AddComponentPurl(c *cyclonedx.Component) {
	if c == nil {
		return
	}
	if c.PackageURL != "" {
		return
	}

	// determine kind.
	kind := "unknown"
	switch c.Type {
	case cyclonedx.ComponentTypeMachineLearningModel:
		kind = "model"
	case cyclonedx.ComponentTypeData:
		kind = "dataset"
	}

	// id from Name.
	id := ""
	if c.Name != "" {
		id = c.Name
	}

	// sha: use first hash value if available.
	sha := ""
	if c.Hashes != nil && len(*c.Hashes) > 0 {
		if (*c.Hashes)[0].Value != "" {
			sha = (*c.Hashes)[0].Value
		}
	}

	// normalize id (preserve slash between namespace and name) and use sha as version.
	rawID := strings.TrimSpace(id)
	var normID string
	if rawID == "" {
		normID = "unknown"
	} else {
		parts := strings.Split(rawID, "/")
		for i, p := range parts {
			parts[i] = NormalizeSegment(strings.TrimSpace(p))
		}
		normID = strings.Join(parts, "/")
	}

	normVersion := strings.ToLower(strings.TrimSpace(sha))
	purl := GeneratePurl(kind, normID, normVersion)
	c.PackageURL = purl
}

// AddComponentBOMRef sets Component.BOMRef. If PURL exists it uses that, otherwise sets a UUID urn.
func AddComponentBOMRef(c *cyclonedx.Component) {
	if c == nil {
		return
	}
	if c.BOMRef != "" {
		return
	}
	if c.PackageURL != "" {
		c.BOMRef = c.PackageURL
		return
	}
	c.BOMRef = "urn:uuid:" + generateUUID()
}
