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
				func(src Source) (any, bool) {
					if s := strings.TrimSpace(src.Scan.Name); s != "" {
						return s, true
					}
					return nil, false
				},
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
					if modelID == "" {
						return nil, false
					}
					input := componentExternalRefsSource{ModelID: modelID}
					if src.Readme != nil {
						input.PaperURL = strings.TrimSpace(src.Readme.PaperURL)
						input.DemoURL = strings.TrimSpace(src.Readme.DemoURL)
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
					if src.HF == nil {
						return nil, false
					}
					lic := extractLicense(src.HF.CardData, src.HF.Tags)
					if lic == "" {
						return nil, false
					}
					return lic, true
				},
				func(src Source) (any, bool) {
					if src.Readme == nil {
						return nil, false
					}
					lic := strings.TrimSpace(src.Readme.License)
					if lic == "" {
						return nil, false
					}
					return lic, true
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
				lic, _ := input.Value.(string)
				lic = strings.TrimSpace(lic)
				if lic == "" {
					return fmt.Errorf("license value is empty")
				}
				if tgt.Component == nil {
					return fmt.Errorf("component is nil")
				}
				if !input.Force && tgt.Component.Licenses != nil && len(*tgt.Component.Licenses) > 0 {
					return nil
				}
				ls := cdx.Licenses{
					{License: &cdx.License{Name: lic}},
				}
				tgt.Component.Licenses = &ls
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
							return s, true
						}
					}
					return nil, false
				},
				func(src Source) (any, bool) {
					if src.Readme != nil {
						if s := strings.TrimSpace(src.Readme.DevelopedBy); s != "" {
							return s, true
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
				s, _ := input.Value.(string)
				s = strings.TrimSpace(s)
				if s == "" {
					return fmt.Errorf("manufacturer value is empty")
				}
				if tgt.Component == nil {
					return fmt.Errorf("component is nil")
				}
				if !input.Force && tgt.Component.Manufacturer != nil && strings.TrimSpace(tgt.Component.Manufacturer.Name) != "" {
					return nil
				}
				tgt.Component.Manufacturer = &cdx.OrganizationalEntity{Name: s}
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
					// Extract group from ModelID (part before /).
					var modelID string
					if src.HF != nil && strings.TrimSpace(src.HF.ModelID) != "" {
						modelID = strings.TrimSpace(src.HF.ModelID)
					} else if src.HF != nil && strings.TrimSpace(src.HF.ID) != "" {
						modelID = strings.TrimSpace(src.HF.ID)
					} else {
						modelID = strings.TrimSpace(src.ModelID)
					}
					if modelID == "" {
						return nil, false
					}
					parts := strings.SplitN(modelID, "/", 2)
					if len(parts) > 0 && strings.TrimSpace(parts[0]) != "" {
						return strings.TrimSpace(parts[0]), true
					}
					return nil, false
				},
				func(src Source) (any, bool) {
					if src.HF != nil {
						if s := strings.TrimSpace(src.HF.Author); s != "" {
							return s, true
						}
					}
					return nil, false
				},
				func(src Source) (any, bool) {
					if src.Readme != nil {
						if s := strings.TrimSpace(src.Readme.DevelopedBy); s != "" {
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
	}
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
