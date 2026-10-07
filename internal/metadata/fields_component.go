package metadata

import (
	"fmt"
	"strings"

	cdx "github.com/CycloneDX/cyclonedx-go"
)

// externalRefsSource is a Hugging Face page (Path below the base URL) plus README links.
type externalRefsSource struct {
	Path     string
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
			Apply: onComponent(applyName),
			Present: func(b *cdx.BOM) bool {
				return bomHasComponentName(b)
			},
			InputType:   InputTypeText,
			Placeholder: "e.g., organization/model-name",
		},
		{
			Key:    ComponentExternalReferences,
			Weight: 0.5,
			Sources: []func(Source) (any, bool){
				func(src Source) (any, bool) {
					modelID := strings.TrimSpace(src.ModelID)
					if src.HF != nil && strings.TrimSpace(src.HF.ID) != "" {
						modelID = strings.TrimSpace(src.HF.ID)
					}
					if modelID == "" {
						return nil, false
					}
					input := externalRefsSource{Path: strings.TrimPrefix(modelID, "/")}
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
			Apply: onComponent(applyExternalRefs),
			Present: func(b *cdx.BOM) bool {
				c := bomComponent(b)
				return c != nil && c.ExternalReferences != nil && len(*c.ExternalReferences) > 0
			},
		},
		{
			Key:    ComponentTags,
			Weight: 0.5,
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
			Apply: onComponent(func(c *cdx.Component, input applyInput, _ string) error {
				tags, _ := input.Value.([]string)
				if len(tags) == 0 {
					return fmt.Errorf("tags value is empty")
				}
				if !input.Force && c.Tags != nil && len(*c.Tags) > 0 {
					return nil
				}
				c.Tags = &tags
				return nil
			}),
			Present: func(b *cdx.BOM) bool {
				c := bomComponent(b)
				return c != nil && c.Tags != nil && len(*c.Tags) > 0
			},
			InputType:   InputTypeMultiText,
			Placeholder: "pytorch, transformers, nlp",
			Suggestions: []string{"pytorch", "transformers", "nlp", "vision", "audio", "text-generation"},
		},
		{
			Key:    ComponentLicenses,
			Weight: 1.0,
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
			Apply: onComponent(applyLicenses),
			Present: func(b *cdx.BOM) bool {
				c := bomComponent(b)
				return c != nil && c.Licenses != nil && len(*c.Licenses) > 0
			},
			InputType:   InputTypeSelect,
			Placeholder: "Select a license",
			Suggestions: []string{"Apache-2.0", "MIT", "BSD-3-Clause", "GPL-3.0", "LGPL-3.0", "CC-BY-4.0", "CC-BY-SA-4.0", "CC0-1.0"},
		},
		{
			Key:    ComponentHashes,
			Weight: 1.0,
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
			Apply: onComponent(applyHash),
			Present: func(b *cdx.BOM) bool {
				c := bomComponent(b)
				return c != nil && c.Hashes != nil && len(*c.Hashes) > 0
			},
			InputType:   InputTypeText,
			Placeholder: "SHA-256 hash value",
		},
		{
			Key:    ComponentManufacturer,
			Weight: 0.5,
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
			Apply: onComponent(applyManufacturer),
			Present: func(b *cdx.BOM) bool {
				c := bomComponent(b)
				return c != nil && c.Manufacturer != nil && strings.TrimSpace(c.Manufacturer.Name) != ""
			},
			InputType:   InputTypeText,
			Placeholder: "Organization or author name",
		},
		{
			Key:    ComponentGroup,
			Weight: 0.25,
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
			Apply: onComponent(applyGroup),
			Present: func(b *cdx.BOM) bool {
				c := bomComponent(b)
				return c != nil && strings.TrimSpace(c.Group) != ""
			},
			InputType:   InputTypeText,
			Placeholder: "Organization or group name",
		},
		{
			Key:    ComponentSupplier,
			Weight: 0.5,
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
			Apply: onComponent(applySupplier),
			Present: func(b *cdx.BOM) bool {
				c := bomComponent(b)
				return c != nil && c.Supplier != nil && strings.TrimSpace(c.Supplier.Name) != ""
			},
			InputType:   InputTypeText,
			Placeholder: "Organization distributing the model",
		},
		{
			Key:    ComponentAuthors,
			Weight: 0.5,
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
			Apply: onComponent(func(c *cdx.Component, input applyInput, _ string) error {
				names, _ := input.Value.([]string)
				var authors []cdx.OrganizationalContact
				for _, n := range normalizeStrings(names) {
					authors = append(authors, cdx.OrganizationalContact{Name: n})
				}
				if len(authors) == 0 {
					return fmt.Errorf("authors value is empty")
				}
				if !input.Force && c.Authors != nil && len(*c.Authors) > 0 {
					return nil
				}
				c.Authors = &authors
				return nil
			}),
			Present: func(b *cdx.BOM) bool {
				c := bomComponent(b)
				return c != nil && c.Authors != nil && len(*c.Authors) > 0
			},
			InputType:   InputTypeMultiText,
			Placeholder: "author1, author2",
		},
		{
			Key:    ComponentVersion,
			Weight: 0.5,
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
			Apply: onComponent(func(c *cdx.Component, input applyInput, _ string) error {
				v, _ := input.Value.(string)
				v = strings.TrimSpace(v)
				if v == "" {
					return fmt.Errorf("version value is empty")
				}
				if !input.Force && strings.TrimSpace(c.Version) != "" {
					return nil
				}
				c.Version = v
				return nil
			}),
			Present: func(b *cdx.BOM) bool {
				c := bomComponent(b)
				return c != nil && strings.TrimSpace(c.Version) != ""
			},
			InputType:   InputTypeText,
			Placeholder: "Model revision (commit SHA or tag)",
		},
		{
			Key:    ComponentDescription,
			Weight: 0.5,
			Sources: []func(Source) (any, bool){
				func(src Source) (any, bool) {
					// Front matter model_description / summary.
					if src.Readme != nil {
						if s := realString(src.Readme.Summary); s != "" {
							return s, true
						}
					}
					return nil, false
				},
				func(src Source) (any, bool) {
					// The same front matter via the API, for when the README fetch failed.
					if src.HF == nil {
						return nil, false
					}
					for _, key := range []string{"model_description", "summary"} {
						s, _ := src.HF.CardData[key].(string)
						if s = realString(strings.Join(strings.Fields(s), " ")); s != "" {
							return s, true
						}
					}
					return nil, false
				},
				func(src Source) (any, bool) {
					// First prose paragraph under a description-like section.
					if src.Readme != nil {
						if s := realString(src.Readme.DescriptionSection); s != "" {
							return s, true
						}
					}
					return nil, false
				},
				func(src Source) (any, bool) {
					// First prose paragraph after the title.
					if src.Readme != nil {
						if s := realString(src.Readme.LeadParagraph); s != "" {
							return s, true
						}
					}
					return nil, false
				},
			},
			Parse: func(value string) (any, error) {
				return parseNonEmptyString(value, "description")
			},
			Apply: onComponent(func(c *cdx.Component, input applyInput, _ string) error {
				v, _ := input.Value.(string)
				v = strings.TrimSpace(v)
				if v == "" {
					return fmt.Errorf("description value is empty")
				}
				if !input.Force && strings.TrimSpace(c.Description) != "" {
					return nil
				}
				c.Description = v
				return nil
			}),
			Present: func(b *cdx.BOM) bool {
				c := bomComponent(b)
				return c != nil && strings.TrimSpace(c.Description) != ""
			},
			InputType:   InputTypeTextArea,
			Placeholder: "One or two sentences describing the model",
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

// Apply funcs shared by the model and dataset registries.

func applyName(c *cdx.Component, input applyInput, _ string) error {
	name, _ := input.Value.(string)
	name = strings.TrimSpace(name)
	if name == "" {
		return fmt.Errorf("name value is empty")
	}
	c.Name = name
	return nil
}

func applyExternalRefs(c *cdx.Component, input applyInput, baseURL string) error {
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
	case externalRefsSource:
		refs = []cdx.ExternalReference{{
			Type: cdx.ExternalReferenceType("website"),
			URL:  hfBaseURL(baseURL) + v.Path,
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
	c.ExternalReferences = &refs
	return nil
}

func applyLicenses(c *cdx.Component, input applyInput, baseURL string) error {
	in, ok := licenseInputFromValue(input.Value)
	if !ok {
		return fmt.Errorf("license value is empty")
	}
	if !input.Force && c.Licenses != nil && len(*c.Licenses) > 0 {
		return nil
	}
	ls := buildLicenses(in, baseURL)
	if ls == nil {
		return fmt.Errorf("license value is a placeholder")
	}
	c.Licenses = ls
	return nil
}

func applyHash(c *cdx.Component, input applyInput, _ string) error {
	sha, _ := input.Value.(string)
	sha = strings.TrimSpace(sha)
	if sha == "" {
		return fmt.Errorf("hash value is empty")
	}
	hs := []cdx.Hash{{Algorithm: cdx.HashAlgoSHA1, Value: sha}}
	c.Hashes = &hs
	return nil
}

func applyGroup(c *cdx.Component, input applyInput, _ string) error {
	s, _ := input.Value.(string)
	s = strings.TrimSpace(s)
	if s == "" {
		return fmt.Errorf("group value is empty")
	}
	if !input.Force && strings.TrimSpace(c.Group) != "" {
		return nil
	}
	c.Group = s
	return nil
}

func applyManufacturer(c *cdx.Component, input applyInput, baseURL string) error {
	return applyOrg(&c.Manufacturer, input, baseURL)
}

func applySupplier(c *cdx.Component, input applyInput, baseURL string) error {
	return applyOrg(&c.Supplier, input, baseURL)
}

func applyOrg(dst **cdx.OrganizationalEntity, input applyInput, baseURL string) error {
	ent, err := organizationalEntity(input.Value, baseURL)
	if err != nil {
		return err
	}
	if !input.Force && *dst != nil && strings.TrimSpace((*dst).Name) != "" {
		return nil
	}
	*dst = ent
	return nil
}
