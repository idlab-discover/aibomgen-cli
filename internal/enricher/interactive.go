package enricher

import (
	"fmt"
	"strings"

	"charm.land/huh/v2"
	cdx "github.com/CycloneDX/cyclonedx-go"
	"github.com/idlab-discover/aibomgen-cli/internal/metadata"
	"github.com/idlab-discover/aibomgen-cli/internal/ui"
)

// runForm runs a form; tests swap it to drive the form non-interactively.
var runForm = ui.RunForm

// formField is the kind-independent view of a model or dataset FieldSpec.
type formField struct {
	title       string
	required    bool
	inputType   metadata.InputType
	placeholder string
	suggestions []string
}

// enrichInteractive enriches fields using interactive forms.
func enrichInteractive(
	bom *cdx.BOM,
	missingFields []metadata.FieldSpec,
	src metadata.Source,
	tgt metadata.Target,
) (map[metadata.Key]string, error) {
	if len(missingFields) == 0 {
		return nil, nil
	}

	fields := make([]formField, len(missingFields))
	for i, spec := range missingFields {
		fields[i] = formField{
			title:       formatTitle(string(spec.Key), spec.Weight, spec.Required),
			required:    spec.Required,
			inputType:   spec.InputType,
			placeholder: spec.Placeholder,
			suggestions: suggestions(spec.Suggestions, spec.Sources, src),
		}
	}

	values, err := runFields("Model Enrichment", "Please provide values for the missing fields.\nPress Enter to skip optional fields.", fields)
	if err != nil {
		return nil, err
	}

	// Apply non-empty values; a value that fails to apply is skipped.
	changes := make(map[metadata.Key]string)
	for i, spec := range missingFields {
		if values[i] != "" && metadata.ApplyUserValue(spec, values[i], tgt) == nil {
			changes[spec.Key] = values[i]
		}
	}
	return changes, nil
}

// enrichDatasetInteractive enriches dataset fields using interactive forms.
func enrichDatasetInteractive(
	comp *cdx.Component,
	missingFields []metadata.DatasetFieldSpec,
	src metadata.DatasetSource,
	tgt metadata.DatasetTarget,
) (map[metadata.DatasetKey]string, error) {
	if len(missingFields) == 0 {
		return nil, nil
	}

	fields := make([]formField, len(missingFields))
	for i, spec := range missingFields {
		fields[i] = formField{
			title:       formatTitle(simplifyDatasetKeyForDisplay(spec.Key), spec.Weight, spec.Required),
			required:    spec.Required,
			inputType:   spec.InputType,
			placeholder: spec.Placeholder,
			suggestions: suggestions(spec.Suggestions, spec.Sources, src),
		}
	}

	values, err := runFields(fmt.Sprintf("Dataset Enrichment: %s", comp.Name), "Please provide values for the missing dataset fields.\nPress Enter to skip optional fields.", fields)
	if err != nil {
		return nil, err
	}

	changes := make(map[metadata.DatasetKey]string)
	for i, spec := range missingFields {
		if values[i] != "" && metadata.ApplyDatasetUserValue(spec, values[i], tgt) == nil {
			changes[spec.Key] = values[i]
		}
	}
	return changes, nil
}

// runFields shows an intro note followed by one group per field and returns
// the entered values in field order.
func runFields(title, description string, fields []formField) ([]string, error) {
	values := make([]string, len(fields))
	groups := []*huh.Group{huh.NewGroup(
		huh.NewNote().
			Title(title).
			Description(description).
			Next(true).
			NextLabel("Continue"),
	)}
	for i, f := range fields {
		groups = append(groups, huh.NewGroup(field(f, &values[i])))
	}
	if err := runForm(huh.NewForm(groups...)); err != nil {
		return nil, err
	}
	return values, nil
}

// field builds the huh input for f according to its InputType.
func field(f formField, value *string) huh.Field {
	validate := func(s string) error {
		if f.required && strings.TrimSpace(s) == "" {
			return fmt.Errorf("this field is required")
		}
		return nil
	}

	switch f.inputType {
	case metadata.InputTypeSelect:
		options := []huh.Option[string]{}
		for _, s := range f.suggestions {
			options = append(options, huh.NewOption(s, s))
		}
		return huh.NewSelect[string]().
			Title(f.title).
			Description(f.placeholder).
			Options(options...).
			Value(value).
			Validate(validate)
	case metadata.InputTypeTextArea:
		return huh.NewText().
			Title(f.title).
			Description(formatDescription(f.suggestions, f.placeholder, false)).
			Placeholder(f.placeholder).
			Value(value).
			Lines(5).
			CharLimit(1000).
			Validate(validate)
	default: // InputTypeText, InputTypeMultiText
		return huh.NewInput().
			Title(f.title).
			Description(formatDescription(f.suggestions, f.placeholder, f.inputType == metadata.InputTypeMultiText)).
			Placeholder(f.placeholder).
			Value(value).
			Validate(validate)
	}
}

// Helper methods.

func formatTitle(name string, weight float64, required bool) string {
	requiredLabel := ""
	if required {
		requiredLabel = ui.Error.Render(" [REQUIRED]")
	}

	weightLabel := ui.Muted.Render(fmt.Sprintf(" (weight: %.1f)", weight))

	return fmt.Sprintf("%s%s%s", name, weightLabel, requiredLabel)
}

func formatDescription(suggestions []string, placeholder string, isArray bool) string {
	var parts []string

	if len(suggestions) > 0 {
		suggestionStr := strings.Join(suggestions, ", ")
		if len(suggestionStr) > 60 {
			suggestionStr = suggestionStr[:60] + "..."
		}
		parts = append(parts, ui.Dim.Render("Suggestions: ")+suggestionStr)
	}

	if isArray {
		parts = append(parts, ui.Muted.Render("Enter comma-separated values"))
	}

	// Always show placeholder hint if not already in the input field placeholder.
	if placeholder != "" && len(parts) > 0 {
		parts = append(parts, ui.Muted.Render("Format: ")+placeholder)
	}

	if len(parts) == 0 {
		parts = append(parts, ui.Muted.Render("Press Enter to skip"))
	}

	return strings.Join(parts, " • ")
}

// suggestions returns explicit when set, else the first non-empty source value.
func suggestions[S any](explicit []string, sources []func(S) (any, bool), src S) []string {
	if len(explicit) > 0 {
		return explicit
	}
	for _, sourceFn := range sources {
		if val, ok := sourceFn(src); ok && val != nil {
			switch v := val.(type) {
			case string:
				if v != "" {
					return []string{v}
				}
			case []string:
				if len(v) > 0 {
					return v
				}
			}
		}
	}
	return nil
}

func camelToTitle(s string) string {
	var result []rune
	for i, r := range s {
		if i > 0 && r >= 'A' && r <= 'Z' {
			result = append(result, ' ')
		}
		result = append(result, r)
	}
	title := string(result)
	// Capitalize first letter of each word.
	words := strings.Fields(title)
	for i, word := range words {
		if len(word) > 0 {
			words[i] = strings.ToUpper(word[:1]) + strings.ToLower(word[1:])
		}
	}
	return strings.Join(words, " ")
}

func simplifyDatasetKeyForDisplay(key metadata.DatasetKey) string {
	keyStr := string(key)
	parts := strings.Split(keyStr, ".")

	if len(parts) == 0 {
		return keyStr
	}

	lastPart := parts[len(parts)-1]

	// Handle special cases.
	if strings.Contains(lastPart, ":") {
		parts := strings.Split(lastPart, ":")
		if len(parts) > 1 {
			lastPart = parts[1]
		}
	}

	return camelToTitle(lastPart)
}
