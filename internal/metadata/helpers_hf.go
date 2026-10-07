package metadata

import (
	"fmt"
	"regexp"
	"strings"

	cdx "github.com/CycloneDX/cyclonedx-go"
	"github.com/idlab-discover/aibomgen-cli/internal/fetcher"
)

// placeholderValues are card-template or "no value" strings that must never be written to a BOM.
// Values are compared after trimming, stripping surrounding brackets and lowercasing.
var placeholderValues = map[string]struct{}{
	"":                        {},
	"unknown":                 {},
	"more information needed": {},
	"needs more information":  {},
	"n/a":                     {},
	"na":                      {},
	"none":                    {},
	"null":                    {},
	"tbd":                     {},
	"todo":                    {},
	"-":                       {},
}

// isPlaceholder reports whether s carries no real value (empty or a known template placeholder).
func isPlaceholder(s string) bool {
	s = strings.TrimSpace(s)
	s = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(s, "["), "]"))
	_, ok := placeholderValues[strings.ToLower(s)]
	return ok
}

// realString returns the trimmed value of s, or "" when s is a placeholder.
func realString(s string) string {
	if isPlaceholder(s) {
		return ""
	}
	return strings.TrimSpace(s)
}

// mdLinkRe matches a value that is exactly one Markdown link: [text](target).
var mdLinkRe = regexp.MustCompile(`^\[([^\]]*)\]\(\s*<?([^)>\s]*)>?\s*\)$`)

// realText returns the trimmed README value s, or "" when s is a placeholder, either
// bare ("[More Information Needed]") or as a Markdown link whose text is a placeholder
// ("[More Information Needed](https://github.com/.../CONTRIBUTING.md)"). It is used for
// values that name an entity (authors, organizations); free README text is kept verbatim.
func realText(s string) string {
	s = strings.TrimSpace(s)
	if m := mdLinkRe.FindStringSubmatch(s); m != nil && isPlaceholder(m[1]) {
		return ""
	}
	return realString(s)
}

// linkURL extracts an http(s) URL from a README bullet value: a bare URL, an
// autolink (<https://...>) or a Markdown link. Placeholders and values that are
// not URLs yield "".
func linkURL(s string) string {
	s = realText(s)
	if m := mdLinkRe.FindStringSubmatch(s); m != nil {
		s = m[2]
	} else if strings.HasPrefix(s, "[") && strings.HasSuffix(s, "]") {
		// Template style "[https://example.com/]": the author replaced the text inside the brackets.
		s = strings.TrimSpace(s[1 : len(s)-1])
	}
	s = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(s, "<"), ">"))
	lower := strings.ToLower(s)
	if (strings.HasPrefix(lower, "https://") || strings.HasPrefix(lower, "http://")) && !strings.ContainsAny(s, " \t\n") {
		return s
	}
	return ""
}

// hfNamespace returns the namespace ("org") of the first id in "org/name" form.
func hfNamespace(ids ...string) (string, bool) {
	for _, id := range ids {
		parts := strings.SplitN(strings.TrimSpace(id), "/", 2)
		if len(parts) != 2 {
			continue
		}
		ns, name := strings.TrimSpace(parts[0]), strings.TrimSpace(parts[1])
		if ns != "" && name != "" {
			return ns, true
		}
	}
	return "", false
}

// hfBaseURL normalizes the Hugging Face base URL (defaults to https://huggingface.co/).
func hfBaseURL(base string) string {
	return fetcher.HFBaseURL(base) + "/"
}

// hfOrgURL returns the Hugging Face profile URL of a namespace.
func hfOrgURL(base, ns string) string {
	return hfBaseURL(base) + strings.TrimSpace(ns)
}

// orgSource is the internal value passed from sources to organizational-entity Apply funcs.
// URL is only set when Name is the Hugging Face namespace itself.
type orgSource struct {
	Name      string
	Namespace string
}

// organizationalEntity builds an entity from a plain string (user input) or an orgSource.
func organizationalEntity(value any, baseURL string) (*cdx.OrganizationalEntity, error) {
	switch v := value.(type) {
	case string:
		name := strings.TrimSpace(v)
		if name == "" {
			return nil, fmt.Errorf("organization name is empty")
		}
		return &cdx.OrganizationalEntity{Name: name}, nil
	case orgSource:
		name := strings.TrimSpace(v.Name)
		if name == "" {
			return nil, fmt.Errorf("organization name is empty")
		}
		ent := &cdx.OrganizationalEntity{Name: name}
		if ns := strings.TrimSpace(v.Namespace); ns != "" && ns == name {
			ent.URL = &[]string{hfOrgURL(baseURL, ns)}
		}
		return ent, nil
	default:
		return nil, fmt.Errorf("invalid organization value")
	}
}

// stringsFromAny flattens a YAML/JSON value that may be a string or a list of strings.
// Values are trimmed; empty entries are dropped (placeholders are kept for callers to filter).
func stringsFromAny(v any) []string {
	var out []string
	switch t := v.(type) {
	case string:
		if s := strings.TrimSpace(t); s != "" {
			out = append(out, s)
		}
	case []string:
		for _, s := range t {
			if s = strings.TrimSpace(s); s != "" {
				out = append(out, s)
			}
		}
	case []any:
		for _, it := range t {
			if s, ok := it.(string); ok {
				if s = strings.TrimSpace(s); s != "" {
					out = append(out, s)
				}
			}
		}
	}
	return out
}

// firstString returns the first non-empty string value among the given maps for key.
func firstString(key string, maps ...map[string]any) string {
	for _, m := range maps {
		if m == nil {
			continue
		}
		if vals := stringsFromAny(m[key]); len(vals) > 0 {
			return vals[0]
		}
	}
	return ""
}

var commitSHARe = regexp.MustCompile(`^[0-9a-fA-F]{40}$`)

// isCommitSHA reports whether rev is a full 40-hex git commit SHA.
func isCommitSHA(rev string) bool {
	return commitSHARe.MatchString(strings.TrimSpace(rev))
}
