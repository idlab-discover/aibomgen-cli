package metadata

import (
	"fmt"
	"strings"

	cdx "github.com/CycloneDX/cyclonedx-go"
)

type componentExternalRefsSource struct {
	ModelID  string
	PaperURL string
	DemoURL  string
}

func componentFields() []FieldSpec {
	return []FieldSpec{
		{
			Key:      ComponentName,
			Weight:   1.0,
			Required: true,
			Sources: []func(Source) (any, bool){
				// Resolved HF ID first: HF redirects renamed/short IDs (gpt2 -> openai-community/gpt2).
				func(src Source) (any, bool) {
					if src.HF == nil {
						return nil, false
					}
					if s := strings.TrimSpace(src.HF.ID); s != "" {
						return s, true
					}
					return nil, false
				},
				func(src Source) (any, bool) {
					if src.HF == nil {
						return nil, false
					}
					if s := strings.TrimSpace(src.HF.ModelID); s != "" {
						return s, true
					}
					return nil, false
				},
				func(src Source) (any, bool) {
					if s := strings.TrimSpace(src.Scan.Name); s != "" {
						return s, true
					}
					return nil, false
				},
				func(src Source) (any, bool) {
					if s := strings.TrimSpace(src.ModelID); s != "" {
						return s, true
					}
					return nil, false
				},
			},
			Parse: func(value string) (any, error) {
				return parseNonEmptyString(value, "name")
			},
			Apply: func(tgt Target, value any) error {
				input, ok := value.(applyInput)
				if !ok {
					return fmt.Errorf("invalid input for %s", ComponentName)
				}
				name, _ := input.Value.(string)
				name = strings.TrimSpace(name)
				if name == "" {
					return fmt.Errorf("name value is empty")
				}
				if tgt.Component == nil {
					return fmt.Errorf("component is nil")
				}
				tgt.Component.Name = name
				return nil
			},
			Present: func(b *cdx.BOM) bool {
				ok := bomHasComponentName(b)
				return ok
			},
			InputType:   InputTypeText,
			Placeholder: "e.g., organization/model-name",
		},
		{
			Key:      ComponentExternalReferences,
			Weight:   0.5,
			Required: false,
			Sources: []func(Source) (any, bool){
				func(src Source) (any, bool) {
					modelID := strings.TrimSpace(src.ModelID)
					if src.HF != nil && strings.TrimSpace(src.HF.ID) != "" {
						modelID = strings.TrimSpace(src.HF.ID)
					}
					if modelID == "" {
						return nil, false
					}
					input := componentExternalRefsSource{ModelID: modelID}
					if src.Readme != nil {
						input.PaperURL = linkURL(src.Readme.PaperURL)
						input.DemoURL = linkURL(src.Readme.DemoURL)
					}
					return input, true
				},
			},
			Parse: func(value string) (any, error) {
				return parseNonEmptyString(value, "externalReferences")
			},
			Apply: func(tgt Target, value any) error {
				input, ok := value.(applyInput)
				if !ok {
					return fmt.Errorf("invalid input for %s", ComponentExternalReferences)
				}
				if tgt.Component == nil {
					return fmt.Errorf("component is nil")
				}

				var refs []cdx.ExternalReference

				switch v := input.Value.(type) {
				case string:
					url := strings.TrimSpace(v)
					if url == "" {
						return fmt.Errorf("externalReferences value is empty")
					}
					refs = []cdx.ExternalReference{{
						Type: cdx.ExternalReferenceType("website"),
						URL:  url,
					}}
				case componentExternalRefsSource:
					base := strings.TrimSpace(tgt.HuggingFaceBaseURL)
					if base == "" {
						base = "https://huggingface.co/"
					}
					if !strings.HasSuffix(base, "/") {
						base += "/"
					}
					url := base + strings.TrimPrefix(v.ModelID, "/")
					refs = []cdx.ExternalReference{{
						Type: cdx.ExternalReferenceType("website"),
						URL:  url,
					}}
					if v.PaperURL != "" {
						refs = append(refs, cdx.ExternalReference{
							Type: cdx.ExternalReferenceType("documentation"),
							URL:  v.PaperURL,
						})
					}
					if v.DemoURL != "" {
						refs = append(refs, cdx.ExternalReference{
							Type: cdx.ExternalReferenceType("other"),
							URL:  v.DemoURL,
						})
					}
				default:
					return fmt.Errorf("invalid externalReferences value")
				}

				tgt.Component.ExternalReferences = &refs
				return nil
			},
			Present: func(b *cdx.BOM) bool {
				c := bomComponent(b)
				ok := c != nil && c.ExternalReferences != nil && len(*c.ExternalReferences) > 0
				return ok
			},
		},
		{
			Key:      ComponentTags,
			Weight:   0.5,
			Required: false,
			Sources: []func(Source) (any, bool){
				func(src Source) (any, bool) {
					if src.HF != nil && len(src.HF.Tags) > 0 {
						tags := normalizeStrings(src.HF.Tags)
						if len(tags) > 0 {
							return tags, true
						}
					}
					return nil, false
				},
				func(src Source) (any, bool) {
					if src.Readme != nil && len(src.Readme.Tags) > 0 {
						tags := normalizeStrings(src.Readme.Tags)
						if len(tags) > 0 {
							return tags, true
						}
					}
					return nil, false
				},
			},
			Parse: func(value string) (any, error) {
				return parseTagsPreserveEmpty(value, "tags")
			},
			Apply: func(tgt Target, value any) error {
				input, ok := value.(applyInput)
				if !ok {
					return fmt.Errorf("invalid input for %s", ComponentTags)
				}
				tags, _ := input.Value.([]string)
				if len(tags) == 0 {
					return fmt.Errorf("tags value is empty")
				}
				if tgt.Component == nil {
					return fmt.Errorf("component is nil")
				}
				if !input.Force && tgt.Component.Tags != nil && len(*tgt.Component.Tags) > 0 {
					return nil
				}
				tgt.Component.Tags = &tags
				return nil
			},
			Present: func(b *cdx.BOM) bool {
				c := bomComponent(b)
				ok := c != nil && c.Tags != nil && len(*c.Tags) > 0
				return ok
			},
			InputType:   InputTypeMultiText,
			Placeholder: "pytorch, transformers, nlp",
			Suggestions: []string{"pytorch", "transformers", "nlp", "vision", "audio", "text-generation"},
		},
		{
			Key:      ComponentLicenses,
			Weight:   1.0,
			Required: false,
			Sources: []func(Source) (any, bool){
				func(src Source) (any, bool) {
					in, ok := modelLicenseInput(src)
					if !ok {
						return nil, false
					}
					return in, true
				},
			},
			Parse: func(value string) (any, error) {
				return parseNonEmptyString(value, "license")
			},
			Apply: func(tgt Target, value any) error {
				input, ok := value.(applyInput)
				if !ok {
					return fmt.Errorf("invalid input for %s", ComponentLicenses)
				}
				in, ok := licenseInputFromValue(input.Value)
				if !ok {
					return fmt.Errorf("license value is empty")
				}
				if tgt.Component == nil {
					return fmt.Errorf("component is nil")
				}
				if !input.Force && tgt.Component.Licenses != nil && len(*tgt.Component.Licenses) > 0 {
					return nil
				}
				ls := buildLicenses(in, tgt.HuggingFaceBaseURL)
				if ls == nil {
					return fmt.Errorf("license value is a placeholder")
				}
				tgt.Component.Licenses = ls
				return nil
			},
			Present: func(b *cdx.BOM) bool {
				c := bomComponent(b)
				ok := c != nil && c.Licenses != nil && len(*c.Licenses) > 0
				return ok
			},
			InputType:   InputTypeSelect,
			Placeholder: "Select a license",
			Suggestions: []string{"Apache-2.0", "MIT", "BSD-3-Clause", "GPL-3.0", "LGPL-3.0", "CC-BY-4.0", "CC-BY-SA-4.0", "CC0-1.0"},
		},
		{
			Key:      ComponentHashes,
			Weight:   1.0,
			Required: false,
			Sources: []func(Source) (any, bool){
				func(src Source) (any, bool) {
					if src.HF == nil {
						return nil, false
					}
					sha := strings.TrimSpace(src.HF.SHA)
					if sha == "" {
						return nil, false
					}
					return sha, true
				},
			},
			Parse: func(value string) (any, error) {
				return parseNonEmptyString(value, "hash")
			},
			Apply: func(tgt Target, value any) error {
				input, ok := value.(applyInput)
				if !ok {
					return fmt.Errorf("invalid input for %s", ComponentHashes)
				}
				sha, _ := input.Value.(string)
				sha = strings.TrimSpace(sha)
				if sha == "" {
					return fmt.Errorf("hash value is empty")
				}
				if tgt.Component == nil {
					return fmt.Errorf("component is nil")
				}
				hs := []cdx.Hash{{Algorithm: cdx.HashAlgoSHA1, Value: sha}}
				tgt.Component.Hashes = &hs
				return nil
			},
			Present: func(b *cdx.BOM) bool {
				c := bomComponent(b)
				ok := c != nil && c.Hashes != nil && len(*c.Hashes) > 0
				return ok
			},
			InputType:   InputTypeText,
			Placeholder: "SHA-256 hash value",
		},
		{
			Key:      ComponentManufacturer,
			Weight:   0.5,
			Required: false,
			Sources: []func(Source) (any, bool){
				func(src Source) (any, bool) {
					if src.HF != nil {
						if s := strings.TrimSpace(src.HF.Author); s != "" {
							ns, _ := modelNamespace(src)
							return orgSource{Name: s, Namespace: ns}, true
						}
					}
					return nil, false
				},
				func(src Source) (any, bool) {
					if src.Readme != nil {
						if s := realText(src.Readme.DevelopedBy); s != "" {
							ns, _ := modelNamespace(src)
							return orgSource{Name: s, Namespace: ns}, true
						}
					}
					return nil, false
				},
			},
			Parse: func(value string) (any, error) {
				return parseNonEmptyString(value, "manufacturer")
			},
			Apply: func(tgt Target, value any) error {
				input, ok := value.(applyInput)
				if !ok {
					return fmt.Errorf("invalid input for %s", ComponentManufacturer)
				}
				if tgt.Component == nil {
					return fmt.Errorf("component is nil")
				}
				ent, err := organizationalEntity(input.Value, tgt.HuggingFaceBaseURL)
				if err != nil {
					return err
				}
				if !input.Force && tgt.Component.Manufacturer != nil && strings.TrimSpace(tgt.Component.Manufacturer.Name) != "" {
					return nil
				}
				tgt.Component.Manufacturer = ent
				return nil
			},
			Present: func(b *cdx.BOM) bool {
				c := bomComponent(b)
				ok := c != nil && c.Manufacturer != nil && strings.TrimSpace(c.Manufacturer.Name) != ""
				return ok
			},
			InputType:   InputTypeText,
			Placeholder: "Organization or author name",
		},
		{
			Key:      ComponentGroup,
			Weight:   0.25,
			Required: false,
			Sources: []func(Source) (any, bool){
				func(src Source) (any, bool) {
					// Namespace of the (resolved) model ID, or the HF author.
					if ns, ok := modelNamespace(src); ok {
						return ns, true
					}
					return nil, false
				},
				func(src Source) (any, bool) {
					if src.Readme != nil {
						if s := realText(src.Readme.DevelopedBy); s != "" {
							return s, true
						}
					}
					return nil, false
				},
			},
			Parse: func(value string) (any, error) {
				return parseNonEmptyString(value, "group")
			},
			Apply: func(tgt Target, value any) error {
				input, ok := value.(applyInput)
				if !ok {
					return fmt.Errorf("invalid input for %s", ComponentGroup)
				}
				s, _ := input.Value.(string)
				s = strings.TrimSpace(s)
				if s == "" {
					return fmt.Errorf("group value is empty")
				}
				if tgt.Component == nil {
					return fmt.Errorf("component is nil")
				}
				if !input.Force && strings.TrimSpace(tgt.Component.Group) != "" {
					return nil
				}
				tgt.Component.Group = s
				return nil
			},
			Present: func(b *cdx.BOM) bool {
				c := bomComponent(b)
				ok := c != nil && strings.TrimSpace(c.Group) != ""
				return ok
			},
			InputType:   InputTypeText,
			Placeholder: "Organization or group name",
		},
		{
			Key:      ComponentSupplier,
			Weight:   0.5,
			Required: false,
			Sources: []func(Source) (any, bool){
				func(src Source) (any, bool) {
					// The Hugging Face namespace distributes the model.
					if ns, ok := modelNamespace(src); ok {
						return orgSource{Name: ns, Namespace: ns}, true
					}
					return nil, false
				},
			},
			Parse: func(value string) (any, error) {
				return parseNonEmptyString(value, "supplier")
			},
			Apply: func(tgt Target, value any) error {
				input, ok := value.(applyInput)
				if !ok {
					return fmt.Errorf("invalid input for %s", ComponentSupplier)
				}
				if tgt.Component == nil {
					return fmt.Errorf("component is nil")
				}
				ent, err := organizationalEntity(input.Value, tgt.HuggingFaceBaseURL)
				if err != nil {
					return err
				}
				if !input.Force && tgt.Component.Supplier != nil && strings.TrimSpace(tgt.Component.Supplier.Name) != "" {
					return nil
				}
				tgt.Component.Supplier = ent
				return nil
			},
			Present: func(b *cdx.BOM) bool {
				c := bomComponent(b)
				return c != nil && c.Supplier != nil && strings.TrimSpace(c.Supplier.Name) != ""
			},
			InputType:   InputTypeText,
			Placeholder: "Organization distributing the model",
		},
		{
			Key:      ComponentAuthors,
			Weight:   0.5,
			Required: false,
			Sources: []func(Source) (any, bool){
				func(src Source) (any, bool) {
					if src.Readme != nil {
						if s := realText(src.Readme.DevelopedBy); s != "" {
							return []string{s}, true
						}
					}
					return nil, false
				},
				func(src Source) (any, bool) {
					// Fallback: the Hugging Face namespace.
					if ns, ok := modelNamespace(src); ok {
						return []string{ns}, true
					}
					return nil, false
				},
			},
			Parse: func(value string) (any, error) {
				return parseCommaList(value, "authors")
			},
			Apply: func(tgt Target, value any) error {
				input, ok := value.(applyInput)
				if !ok {
					return fmt.Errorf("invalid input for %s", ComponentAuthors)
				}
				if tgt.Component == nil {
					return fmt.Errorf("component is nil")
				}
				names, _ := input.Value.([]string)
				var authors []cdx.OrganizationalContact
				for _, n := range normalizeStrings(names) {
					authors = append(authors, cdx.OrganizationalContact{Name: n})
				}
				if len(authors) == 0 {
					return fmt.Errorf("authors value is empty")
				}
				if !input.Force && tgt.Component.Authors != nil && len(*tgt.Component.Authors) > 0 {
					return nil
				}
				tgt.Component.Authors = &authors
				return nil
			},
			Present: func(b *cdx.BOM) bool {
				c := bomComponent(b)
				return c != nil && c.Authors != nil && len(*c.Authors) > 0
			},
			InputType:   InputTypeMultiText,
			Placeholder: "author1, author2",
		},
		{
			Key:      ComponentVersion,
			Weight:   0.5,
			Required: false,
			Sources: []func(Source) (any, bool){
				func(src Source) (any, bool) {
					// A named revision (branch/tag) is the version the user asked for.
					if rev := strings.TrimSpace(src.Revision); rev != "" && !isCommitSHA(rev) {
						return rev, true
					}
					return nil, false
				},
				func(src Source) (any, bool) {
					// Otherwise the resolved commit, identical to the purl version.
					if src.HF != nil {
						if sha := strings.ToLower(strings.TrimSpace(src.HF.SHA)); sha != "" {
							return sha, true
						}
					}
					return nil, false
				},
			},
			Parse: func(value string) (any, error) {
				return parseNonEmptyString(value, "version")
			},
			Apply: func(tgt Target, value any) error {
				input, ok := value.(applyInput)
				if !ok {
					return fmt.Errorf("invalid input for %s", ComponentVersion)
				}
				v, _ := input.Value.(string)
				v = strings.TrimSpace(v)
				if v == "" {
					return fmt.Errorf("version value is empty")
				}
				if tgt.Component == nil {
					return fmt.Errorf("component is nil")
				}
				if !input.Force && strings.TrimSpace(tgt.Component.Version) != "" {
					return nil
				}
				tgt.Component.Version = v
				return nil
			},
			Present: func(b *cdx.BOM) bool {
				c := bomComponent(b)
				return c != nil && strings.TrimSpace(c.Version) != ""
			},
			InputType:   InputTypeText,
			Placeholder: "Model revision (commit SHA or tag)",
		},
	}
}

// modelNamespace returns the Hugging Face namespace of the model:.
// the org of the resolved ID (HF.ID, HF.ModelID) or requested ID, else the HF author.
func modelNamespace(src Source) (string, bool) {
	var ids []string
	if src.HF != nil {
		ids = append(ids, src.HF.ID, src.HF.ModelID)
	}
	ids = append(ids, src.ModelID)
	if ns, ok := hfNamespace(ids...); ok {
		return ns, true
	}
	if src.HF != nil {
		if a := strings.TrimSpace(src.HF.Author); a != "" {
			return a, true
		}
	}
	return "", false
}

func evidenceFields() []FieldSpec {
	return []FieldSpec{
		{
			Key:      Key("aibomgen.evidence"),
			Weight:   0,
			Required: false,
			Sources: []func(Source) (any, bool){
				func(src Source) (any, bool) {
					return src, true
				},
			},
			Apply: func(tgt Target, value any) error {
				input, ok := value.(applyInput)
				if !ok {
					return fmt.Errorf("invalid input for aibomgen.evidence")
				}
				src, ok := input.Value.(Source)
				if !ok {
					return fmt.Errorf("invalid evidence value")
				}
				if tgt.Component == nil || !tgt.IncludeEvidenceProperties {
					return nil
				}
				setProperty(tgt.Component, "aibomgen.type", src.Scan.Type)
				setProperty(tgt.Component, "aibomgen.evidence", src.Scan.Evidence)
				setProperty(tgt.Component, "aibomgen.path", src.Scan.Path)
				return nil
			},
			Present: func(b *cdx.BOM) bool {
				return true
			},
		},
	}
}
