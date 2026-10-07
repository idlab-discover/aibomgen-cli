package metadata

import (
	"fmt"
	"strings"

	cdx "github.com/CycloneDX/cyclonedx-go"
)

type datasetExternalRefsSource struct {
	DatasetID string
	PaperURL  string
	DemoURL   string
}

// datasetNamespace returns the Hugging Face namespace of the dataset:.
// the org of the resolved (HF.ID) or requested ID, else the HF author.
func datasetNamespace(src DatasetSource) (string, bool) {
	var ids []string
	if src.HF != nil {
		ids = append(ids, src.HF.ID)
	}
	ids = append(ids, src.DatasetID)
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

// DatasetRegistry returns all dataset field specifications.
func DatasetRegistry() []DatasetFieldSpec {
	return []DatasetFieldSpec{
		{
			Key:      DatasetName,
			Weight:   1.0,
			Required: true,
			Sources: []func(DatasetSource) (any, bool){
				// Resolved HF ID first: HF redirects renamed datasets (wikipedia -> legacy-datasets/wikipedia).
				func(src DatasetSource) (any, bool) {
					if src.HF == nil {
						return nil, false
					}
					if s := strings.TrimSpace(src.HF.ID); s != "" {
						return s, true
					}
					return nil, false
				},
				func(src DatasetSource) (any, bool) {
					if s := strings.TrimSpace(src.Scan.Name); s != "" {
						return s, true
					}
					return nil, false
				},
				func(src DatasetSource) (any, bool) {
					if s := strings.TrimSpace(src.DatasetID); s != "" {
						return s, true
					}
					return nil, false
				},
			},
			Parse: func(value string) (any, error) {
				return parseNonEmptyString(value, "name")
			},
			Apply: func(tgt DatasetTarget, value any) error {
				input, ok := value.(applyInput)
				if !ok {
					return fmt.Errorf("invalid input for %s", DatasetName)
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
			Present: func(comp *cdx.Component) bool {
				return comp != nil && strings.TrimSpace(comp.Name) != ""
			},
			InputType:   InputTypeText,
			Placeholder: "e.g., organization/dataset-name",
		},
		{
			Key:    DatasetExternalReferences,
			Weight: 0.5,
			Sources: []func(DatasetSource) (any, bool){
				func(src DatasetSource) (any, bool) {
					datasetID := strings.TrimSpace(src.DatasetID)
					if src.HF != nil && strings.TrimSpace(src.HF.ID) != "" {
						datasetID = strings.TrimSpace(src.HF.ID)
					}
					if datasetID == "" {
						return nil, false
					}
					input := datasetExternalRefsSource{DatasetID: datasetID}
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
			Apply: func(tgt DatasetTarget, value any) error {
				input, ok := value.(applyInput)
				if !ok {
					return fmt.Errorf("invalid input for %s", DatasetExternalReferences)
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
				case datasetExternalRefsSource:
					base := strings.TrimSpace(tgt.HuggingFaceBaseURL)
					if base == "" {
						base = "https://huggingface.co/"
					}
					if !strings.HasSuffix(base, "/") {
						base += "/"
					}
					url := base + "datasets/" + strings.TrimPrefix(v.DatasetID, "/")
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
			Present: func(comp *cdx.Component) bool {
				return comp != nil && comp.ExternalReferences != nil && len(*comp.ExternalReferences) > 0
			},
			InputType:   InputTypeText,
			Placeholder: "https://example.com/dataset",
		},
		{
			Key:    DatasetTags,
			Weight: 0.5,
			Sources: []func(DatasetSource) (any, bool){
				func(src DatasetSource) (any, bool) {
					if src.HF != nil && len(src.HF.Tags) > 0 {
						tags := normalizeStrings(src.HF.Tags)
						if len(tags) > 0 {
							return tags, true
						}
					}
					return nil, false
				},
				func(src DatasetSource) (any, bool) {
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
				parts := strings.Split(value, ",")
				tags := normalizeStrings(parts)
				return tags, nil
			},
			Apply: func(tgt DatasetTarget, value any) error {
				input, ok := value.(applyInput)
				if !ok {
					return fmt.Errorf("invalid input for %s", DatasetTags)
				}
				tags, _ := input.Value.([]string)
				if tgt.Component == nil {
					return fmt.Errorf("component is nil")
				}
				if !input.Force && tgt.Component.Tags != nil && len(*tgt.Component.Tags) > 0 {
					return nil
				}
				tgt.Component.Tags = &tags
				return nil
			},
			Present: func(comp *cdx.Component) bool {
				return comp != nil && comp.Tags != nil && len(*comp.Tags) > 0
			},
			InputType:   InputTypeMultiText,
			Placeholder: "nlp, text, en",
			Suggestions: []string{"nlp", "vision", "audio", "tabular", "multimodal", "text", "image"},
		},
		{
			Key:    DatasetLicenses,
			Weight: 0.8,
			Sources: []func(DatasetSource) (any, bool){
				func(src DatasetSource) (any, bool) {
					in, ok := datasetLicenseInput(src)
					if !ok {
						return nil, false
					}
					return in, true
				},
			},
			Parse: func(value string) (any, error) {
				return parseNonEmptyString(value, "license")
			},
			Apply: func(tgt DatasetTarget, value any) error {
				input, ok := value.(applyInput)
				if !ok {
					return fmt.Errorf("invalid input for %s", DatasetLicenses)
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
			Present: func(comp *cdx.Component) bool {
				return comp != nil && comp.Licenses != nil && len(*comp.Licenses) > 0
			},
			InputType:   InputTypeSelect,
			Placeholder: "Select a license",
			Suggestions: []string{"Apache-2.0", "MIT", "CC-BY-4.0", "CC-BY-SA-4.0", "CC0-1.0"},
		},
		{
			Key:    DatasetDescription,
			Weight: 0.7,
			Sources: []func(DatasetSource) (any, bool){
				func(src DatasetSource) (any, bool) {
					if src.Readme != nil {
						desc := strings.TrimSpace(src.Readme.DatasetDescription)
						if desc != "" {
							return desc, true
						}
					}
					return nil, false
				},
				func(src DatasetSource) (any, bool) {
					if src.HF != nil {
						desc := strings.TrimSpace(src.HF.Description)
						if desc != "" {
							return desc, true
						}
					}
					return nil, false
				},
			},
			Parse: func(value string) (any, error) {
				return parseNonEmptyString(value, "description")
			},
			Apply: func(tgt DatasetTarget, value any) error {
				input, ok := value.(applyInput)
				if !ok {
					return fmt.Errorf("invalid input for %s", DatasetDescription)
				}
				desc, _ := input.Value.(string)
				desc = strings.TrimSpace(desc)
				if desc == "" {
					return fmt.Errorf("description value is empty")
				}
				if tgt.Component == nil {
					return fmt.Errorf("component is nil")
				}
				data := ensureComponentData(tgt.Component)
				data.Description = desc
				return nil
			},
			Present: func(comp *cdx.Component) bool {
				data := getComponentData(comp)
				return data != nil && strings.TrimSpace(data.Description) != ""
			},
			InputType:   InputTypeTextArea,
			Placeholder: "Describe the dataset...",
		},
		{
			Key:    DatasetManufacturer,
			Weight: 0.6,
			Sources: []func(DatasetSource) (any, bool){
				func(src DatasetSource) (any, bool) {
					ns, _ := datasetNamespace(src)
					// First try API author (authors[0]).
					if src.HF != nil && strings.TrimSpace(src.HF.Author) != "" {
						return orgSource{Name: strings.TrimSpace(src.HF.Author), Namespace: ns}, true
					}
					// Fallback to first AnnotationCreator from README (authors[1]).
					if src.Readme != nil && len(src.Readme.AnnotationCreators) > 0 {
						if trimmed := strings.TrimSpace(src.Readme.AnnotationCreators[0]); trimmed != "" {
							return orgSource{Name: trimmed, Namespace: ns}, true
						}
					}
					return nil, false
				},
			},
			Parse: func(value string) (any, error) {
				return parseNonEmptyString(value, "manufacturer")
			},
			Apply: func(tgt DatasetTarget, value any) error {
				input, ok := value.(applyInput)
				if !ok {
					return fmt.Errorf("invalid input for %s", DatasetManufacturer)
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
			Present: func(comp *cdx.Component) bool {
				return comp != nil && comp.Manufacturer != nil && strings.TrimSpace(comp.Manufacturer.Name) != ""
			},
			InputType:   InputTypeText,
			Placeholder: "Organization or author name",
		},
		{
			Key:    DatasetSupplier,
			Weight: 0.4,
			Sources: []func(DatasetSource) (any, bool){
				func(src DatasetSource) (any, bool) {
					// The Hugging Face namespace distributes the dataset.
					if ns, ok := datasetNamespace(src); ok {
						return orgSource{Name: ns, Namespace: ns}, true
					}
					return nil, false
				},
			},
			Parse: func(value string) (any, error) {
				return parseNonEmptyString(value, "supplier")
			},
			Apply: func(tgt DatasetTarget, value any) error {
				input, ok := value.(applyInput)
				if !ok {
					return fmt.Errorf("invalid input for %s", DatasetSupplier)
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
			Present: func(comp *cdx.Component) bool {
				return comp != nil && comp.Supplier != nil && strings.TrimSpace(comp.Supplier.Name) != ""
			},
			InputType:   InputTypeText,
			Placeholder: "Organization distributing the dataset",
		},
		{
			Key:    DatasetAuthors,
			Weight: 0.6,
			Sources: []func(DatasetSource) (any, bool){
				func(src DatasetSource) (any, bool) {
					var allAuthors []string

					// First, add API author if available.
					if src.HF != nil && strings.TrimSpace(src.HF.Author) != "" {
						allAuthors = append(allAuthors, strings.TrimSpace(src.HF.Author))
					}

					// Then, add annotation creators from README.
					if src.Readme != nil && len(src.Readme.AnnotationCreators) > 0 {
						for _, creator := range src.Readme.AnnotationCreators {
							if trimmed := strings.TrimSpace(creator); trimmed != "" {
								allAuthors = append(allAuthors, trimmed)
							}
						}
					}

					if len(allAuthors) == 0 {
						return nil, false
					}
					return allAuthors, true
				},
			},
			Parse: func(value string) (any, error) {
				parts := strings.Split(value, ",")
				authors := normalizeStrings(parts)
				return authors, nil
			},
			Apply: func(tgt DatasetTarget, value any) error {
				input, ok := value.(applyInput)
				if !ok {
					return fmt.Errorf("invalid input for %s", DatasetAuthors)
				}
				if tgt.Component == nil {
					return fmt.Errorf("component is nil")
				}
				var authors []cdx.OrganizationalContact
				switch v := input.Value.(type) {
				case []string:
					for _, authorName := range v {
						if trimmed := strings.TrimSpace(authorName); trimmed != "" {
							authors = append(authors, cdx.OrganizationalContact{
								Name: trimmed,
							})
						}
					}
				case string:
					if trimmed := strings.TrimSpace(v); trimmed != "" {
						authors = append(authors, cdx.OrganizationalContact{
							Name: trimmed,
						})
					}
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
			Present: func(comp *cdx.Component) bool {
				return comp != nil && comp.Authors != nil && len(*comp.Authors) > 0
			},
			InputType:   InputTypeMultiText,
			Placeholder: "author1, author2, author3",
		},
		{
			Key:    DatasetGroup,
			Weight: 0.4,
			Sources: []func(DatasetSource) (any, bool){
				func(src DatasetSource) (any, bool) {
					// Namespace of the (resolved) dataset ID, or the HF author.
					if ns, ok := datasetNamespace(src); ok {
						return ns, true
					}
					return nil, false
				},
			},
			Parse: func(value string) (any, error) {
				return parseNonEmptyString(value, "group")
			},
			Apply: func(tgt DatasetTarget, value any) error {
				input, ok := value.(applyInput)
				if !ok {
					return fmt.Errorf("invalid input for %s", DatasetGroup)
				}
				group, _ := input.Value.(string)
				group = strings.TrimSpace(group)
				if group == "" {
					return fmt.Errorf("group value is empty")
				}
				if tgt.Component == nil {
					return fmt.Errorf("component is nil")
				}
				if !input.Force && strings.TrimSpace(tgt.Component.Group) != "" {
					return nil
				}
				tgt.Component.Group = group
				return nil
			},
			Present: func(comp *cdx.Component) bool {
				return comp != nil && strings.TrimSpace(comp.Group) != ""
			},
			InputType:   InputTypeText,
			Placeholder: "Organization or group name",
		},
		{
			Key:    DatasetContents,
			Weight: 0.5,
			Sources: []func(DatasetSource) (any, bool){
				func(src DatasetSource) (any, bool) {
					if src.Readme == nil {
						return nil, false
					}
					if len(src.Readme.Configs) == 0 {
						return nil, false
					}
					var contentParts []string
					for _, config := range src.Readme.Configs {
						for _, df := range config.DataFiles {
							contentParts = append(contentParts, fmt.Sprintf("config:%s split:%s path:%s", config.Name, df.Split, df.Path))
						}
					}
					if len(contentParts) == 0 {
						return nil, false
					}
					return strings.Join(contentParts, "\n"), true
				},
			},
			Apply: func(tgt DatasetTarget, value any) error {
				input, ok := value.(applyInput)
				if !ok {
					return fmt.Errorf("invalid input for %s", DatasetContents)
				}
				content, _ := input.Value.(string)
				if tgt.Component == nil {
					return fmt.Errorf("component is nil")
				}
				if strings.TrimSpace(content) == "" {
					return nil
				}
				data := ensureComponentData(tgt.Component)
				if data.Contents == nil {
					data.Contents = &cdx.ComponentDataContents{}
				}
				data.Contents.Attachment = &cdx.AttachedText{
					Content:     content,
					ContentType: "text/plain",
				}
				return nil
			},
			Present: func(comp *cdx.Component) bool {
				data := getComponentData(comp)
				return data != nil && data.Contents != nil && data.Contents.Attachment != nil
			},
			InputType:   InputTypeTextArea,
			Placeholder: "Describe dataset contents...",
		},
		{
			Key:    DatasetSensitiveData,
			Weight: 0.6,
			Sources: []func(DatasetSource) (any, bool){
				func(src DatasetSource) (any, bool) {
					var sensitiveItems []string
					if src.HF != nil && src.HF.CardData != nil {
						if tagsData, ok := src.HF.CardData["tags"]; ok {
							if tags, ok := tagsData.([]interface{}); ok {
								for _, tag := range tags {
									if tagStr, ok := tag.(string); ok {
										sensitiveItems = append(sensitiveItems, tagStr)
									}
								}
							}
						}
					}
					if src.Readme != nil {
						if out := strings.TrimSpace(src.Readme.OutOfScopeUse); out != "" {
							sensitiveItems = append(sensitiveItems, "out-of-scope: "+out)
						}
						if psi := strings.TrimSpace(src.Readme.PersonalSensitiveInfo); psi != "" {
							sensitiveItems = append(sensitiveItems, "personal-info: "+psi)
						}
						if brl := strings.TrimSpace(src.Readme.BiasRisksLimitations); brl != "" {
							sensitiveItems = append(sensitiveItems, "bias-risks: "+brl)
						}
					}
					if len(sensitiveItems) == 0 {
						return nil, false
					}
					return sensitiveItems, true
				},
			},
			Parse: func(value string) (any, error) {
				return parseNonEmptyString(value, "sensitive data")
			},
			Apply: func(tgt DatasetTarget, value any) error {
				input, ok := value.(applyInput)
				if !ok {
					return fmt.Errorf("invalid input for %s", DatasetSensitiveData)
				}
				if tgt.Component == nil {
					return fmt.Errorf("component is nil")
				}
				items := []string{}
				switch v := input.Value.(type) {
				case string:
					if strings.TrimSpace(v) == "" {
						return fmt.Errorf("sensitive data value is empty")
					}
					items = []string{v}
				case []string:
					items = v
				}
				if len(items) == 0 {
					return fmt.Errorf("sensitive data value is empty")
				}
				data := ensureComponentData(tgt.Component)
				data.SensitiveData = &items
				return nil
			},
			Present: func(comp *cdx.Component) bool {
				data := getComponentData(comp)
				return data != nil && data.SensitiveData != nil && len(*data.SensitiveData) > 0
			},
			InputType:   InputTypeTextArea,
			Placeholder: "Describe any sensitive data...",
		},
		{
			Key:    DatasetClassification,
			Weight: 0.6,
			Sources: []func(DatasetSource) (any, bool){
				func(src DatasetSource) (any, bool) {
					if src.HF != nil && src.HF.CardData != nil {
						if taskCats, ok := src.HF.CardData["task_categories"]; ok {
							if cats, ok := taskCats.([]interface{}); ok && len(cats) > 0 {
								if cat, ok := cats[0].(string); ok {
									return cat, true
								}
							}
						}
					}
					return nil, false
				},
			},
			Parse: func(value string) (any, error) {
				return parseNonEmptyString(value, "classification")
			},
			Apply: func(tgt DatasetTarget, value any) error {
				input, ok := value.(applyInput)
				if !ok {
					return fmt.Errorf("invalid input for %s", DatasetClassification)
				}
				classification, _ := input.Value.(string)
				classification = strings.TrimSpace(classification)
				if classification == "" {
					return fmt.Errorf("classification value is empty")
				}
				if tgt.Component == nil {
					return fmt.Errorf("component is nil")
				}
				data := ensureComponentData(tgt.Component)
				data.Classification = classification
				return nil
			},
			Present: func(comp *cdx.Component) bool {
				data := getComponentData(comp)
				return data != nil && strings.TrimSpace(data.Classification) != ""
			},
			InputType:   InputTypeText,
			Placeholder: "text, image, audio, etc.",
			Suggestions: []string{"text", "image", "audio", "video", "tabular"},
		},
		{
			Key:    DatasetGovernance,
			Weight: 0.7,
			Sources: []func(DatasetSource) (any, bool){
				func(src DatasetSource) (any, bool) {
					governance := &cdx.DataGovernance{}
					hasGovernance := false
					var custodianName string
					if src.HF != nil && strings.TrimSpace(src.HF.Author) != "" {
						custodianName = strings.TrimSpace(src.HF.Author)
					} else if src.Readme != nil {
						if realText(src.Readme.SharedBy) != "" {
							custodianName = realText(src.Readme.SharedBy)
						} else if realText(src.Readme.CuratedBy) != "" {
							custodianName = realText(src.Readme.CuratedBy)
						}
					}
					if custodianName != "" {
						governance.Custodians = &[]cdx.ComponentDataGovernanceResponsibleParty{{
							Organization: &cdx.OrganizationalEntity{Name: custodianName},
						}}
						hasGovernance = true
					}
					if src.Readme != nil && realText(src.Readme.CuratedBy) != "" {
						governance.Stewards = &[]cdx.ComponentDataGovernanceResponsibleParty{{
							Organization: &cdx.OrganizationalEntity{Name: realText(src.Readme.CuratedBy)},
						}}
						hasGovernance = true
					}
					if src.Readme != nil && realText(src.Readme.FundedBy) != "" {
						governance.Owners = &[]cdx.ComponentDataGovernanceResponsibleParty{{
							Organization: &cdx.OrganizationalEntity{Name: realText(src.Readme.FundedBy)},
						}}
						hasGovernance = true
					}
					if !hasGovernance {
						return nil, false
					}
					return governance, true
				},
			},
			Parse: func(value string) (any, error) {
				return parseDataGovernance(value)
			},
			Apply: func(tgt DatasetTarget, value any) error {
				input, ok := value.(applyInput)
				if !ok {
					return fmt.Errorf("invalid input for %s", DatasetGovernance)
				}
				gov, _ := input.Value.(*cdx.DataGovernance)
				if gov == nil {
					return fmt.Errorf("governance value is nil")
				}
				if tgt.Component == nil {
					return fmt.Errorf("component is nil")
				}
				data := ensureComponentData(tgt.Component)
				data.Governance = gov
				return nil
			},
			Present: func(comp *cdx.Component) bool {
				data := getComponentData(comp)
				return data != nil && data.Governance != nil
			},
			InputType:   InputTypeTextArea,
			Placeholder: "custodian:OrgName,steward:CuratorName,owner:FunderName",
		},
		{
			Key:    DatasetHashes,
			Weight: 0.5,
			Sources: []func(DatasetSource) (any, bool){
				func(src DatasetSource) (any, bool) {
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
			Apply: func(tgt DatasetTarget, value any) error {
				input, ok := value.(applyInput)
				if !ok {
					return fmt.Errorf("invalid input for %s", DatasetHashes)
				}
				hash, _ := input.Value.(string)
				hash = strings.TrimSpace(hash)
				if hash == "" {
					return fmt.Errorf("hash value is empty")
				}
				if tgt.Component == nil {
					return fmt.Errorf("component is nil")
				}
				hashes := []cdx.Hash{{
					Algorithm: cdx.HashAlgoSHA1,
					Value:     hash,
				}}
				tgt.Component.Hashes = &hashes
				return nil
			},
			Present: func(comp *cdx.Component) bool {
				return comp != nil && comp.Hashes != nil && len(*comp.Hashes) > 0
			},
			InputType:   InputTypeText,
			Placeholder: "SHA-256 hash value",
		},
		{
			Key:    DatasetCreatedAt,
			Weight: 0.3,
			Sources: []func(DatasetSource) (any, bool){
				func(src DatasetSource) (any, bool) {
					if src.HF == nil {
						return nil, false
					}
					createdAt := strings.TrimSpace(src.HF.CreatedAt)
					if createdAt == "" {
						return nil, false
					}
					return createdAt, true
				},
			},
			Parse: func(value string) (any, error) {
				return parseOptionalString(value)
			},
			Apply: func(tgt DatasetTarget, value any) error {
				input, ok := value.(applyInput)
				if !ok {
					return fmt.Errorf("invalid input for %s", DatasetCreatedAt)
				}
				if tgt.Component == nil {
					return fmt.Errorf("component is nil")
				}
				createdAt, _ := input.Value.(string)
				setProperty(tgt.Component, "huggingface:createdAt", strings.TrimSpace(createdAt))
				return nil
			},
			Present: func(comp *cdx.Component) bool {
				return hasProperty(comp, "huggingface:createdAt")
			},
			InputType:   InputTypeText,
			Placeholder: "YYYY-MM-DD",
		},
		{
			Key:    DatasetUsedStorage,
			Weight: 0.3,
			Sources: []func(DatasetSource) (any, bool){
				func(src DatasetSource) (any, bool) {
					if src.HF == nil || src.HF.UsedStorage <= 0 {
						return nil, false
					}
					return fmt.Sprintf("%d", src.HF.UsedStorage), true
				},
			},
			Parse: func(value string) (any, error) {
				return parseOptionalString(value)
			},
			Apply: func(tgt DatasetTarget, value any) error {
				input, ok := value.(applyInput)
				if !ok {
					return fmt.Errorf("invalid input for %s", DatasetUsedStorage)
				}
				if tgt.Component == nil {
					return fmt.Errorf("component is nil")
				}
				usedStorage, _ := input.Value.(string)
				setProperty(tgt.Component, "huggingface:usedStorage", strings.TrimSpace(usedStorage))
				return nil
			},
			Present: func(comp *cdx.Component) bool {
				return hasProperty(comp, "huggingface:usedStorage")
			},
			InputType:   InputTypeText,
			Placeholder: "Storage size in bytes",
		},
		{
			Key:    DatasetLastModified,
			Weight: 0.3,
			Sources: []func(DatasetSource) (any, bool){
				func(src DatasetSource) (any, bool) {
					if src.HF == nil {
						return nil, false
					}
					lastMod := strings.TrimSpace(src.HF.LastMod)
					if lastMod == "" {
						return nil, false
					}
					return lastMod, true
				},
			},
			Parse: func(value string) (any, error) {
				return parseNonEmptyString(value, "lastModified")
			},
			Apply: func(tgt DatasetTarget, value any) error {
				input, ok := value.(applyInput)
				if !ok {
					return fmt.Errorf("invalid input for %s", DatasetLastModified)
				}
				lastMod, _ := input.Value.(string)
				lastMod = strings.TrimSpace(lastMod)
				if lastMod == "" {
					return fmt.Errorf("lastModified value is empty")
				}
				if tgt.Component == nil {
					return fmt.Errorf("component is nil")
				}
				if tgt.Component.Tags != nil {
					tags := *tgt.Component.Tags
					tags = append(tags, "lastModified:"+lastMod)
					tgt.Component.Tags = &tags
				} else {
					tags := []string{"lastModified:" + lastMod}
					tgt.Component.Tags = &tags
				}
				return nil
			},
			Present: func(comp *cdx.Component) bool {
				if comp == nil || comp.Tags == nil {
					return false
				}
				for _, tag := range *comp.Tags {
					if strings.HasPrefix(tag, "lastModified:") {
						return true
					}
				}
				return false
			},
			InputType:   InputTypeText,
			Placeholder: "YYYY-MM-DD",
		},
		{
			Key:    DatasetContact,
			Weight: 0.5,
			Sources: []func(DatasetSource) (any, bool){
				func(src DatasetSource) (any, bool) {
					if src.Readme == nil {
						return nil, false
					}
					contact := strings.TrimSpace(src.Readme.DatasetCardContact)
					if contact == "" {
						return nil, false
					}
					return contact, true
				},
			},
			Parse: func(value string) (any, error) {
				return parseNonEmptyString(value, "contact")
			},
			Apply: func(tgt DatasetTarget, value any) error {
				input, ok := value.(applyInput)
				if !ok {
					return fmt.Errorf("invalid input for %s", DatasetContact)
				}
				contact, _ := input.Value.(string)
				contact = strings.TrimSpace(contact)
				if contact == "" {
					return fmt.Errorf("contact value is empty")
				}
				if tgt.Component == nil {
					return fmt.Errorf("component is nil")
				}
				setProperty(tgt.Component, "huggingface:datasetContact", contact)
				return nil
			},
			Present: func(comp *cdx.Component) bool {
				return hasProperty(comp, "huggingface:datasetContact")
			},
			InputType:   InputTypeText,
			Placeholder: "Contact information",
		},
	}
}
