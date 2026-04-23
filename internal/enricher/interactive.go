// File: internal/enricher/interactive.go - Replace the entire file with this corrected version.

package enricher

import (
	"fmt"
	"strings"

	"charm.land/huh/v2"
	cdx "github.com/CycloneDX/cyclonedx-go"
	"github.com/idlab-discover/aibomgen-cli/internal/metadata"
	"github.com/idlab-discover/aibomgen-cli/internal/ui"
)

// InteractiveEnricher provides a form-based interactive enrichment experience.
type InteractiveEnricher struct {
	enricher *Enricher
}

// NewInteractiveEnricher creates a new InteractiveEnricher instance.
func NewInteractiveEnricher(e *Enricher) *InteractiveEnricher {
	return &InteractiveEnricher{
		enricher: e,
	}
}

// EnrichInteractive enriches fields using interactive forms.
func (ie *InteractiveEnricher) EnrichInteractive(
	bom *cdx.BOM,
	missingFields []metadata.FieldSpec,
	src metadata.Source,
	tgt metadata.Target,
) (map[metadata.Key]string, error) {
	if len(missingFields) == 0 {
		return nil, nil
	}

	// Storage for form values - use map of pointers.
	valueStore := make(map[metadata.Key]*string)
	for _, spec := range missingFields {
		val := ""
		valueStore[spec.Key] = &val
	}

	// Create form groups - one form with all fields.
	formGroups := []*huh.Group{}

	// Add intro note.
	formGroups = append(formGroups, huh.NewGroup(
		huh.NewNote().
			Title("Model Enrichment").
			Description("Please provide values for the missing fields.\nPress Enter to skip optional fields.").
			Next(true).
			NextLabel("Continue"),
	))

	// Create inputs for each field.
	for _, spec := range missingFields {
		fieldInputs := ie.createFieldInput(spec, src, valueStore[spec.Key])
		if len(fieldInputs) > 0 {
			formGroups = append(formGroups, huh.NewGroup(fieldInputs...))
		}
	}

	// Skip if no inputs were created.
	if len(formGroups) <= 1 {
		return nil, nil
	}

	// Create and run form.
	form := huh.NewForm(formGroups...)
	err := form.Run()
	if err != nil {
		return nil, err
	}

	// Now read the values from the pointers and apply them.
	changes := make(map[metadata.Key]string)
	for _, spec := range missingFields {
		strValue := *valueStore[spec.Key]
		if strValue == "" {
			continue
		}

		// Apply the value.
		err := metadata.ApplyUserValue(spec, strValue, tgt)
		if err != nil {
			// Continue on error, just skip this field.
			continue
		}
		changes[spec.Key] = strValue
	}

	return changes, nil
}

// createFieldInput creates form inputs for a field spec.
func (ie *InteractiveEnricher) createFieldInput(
	spec metadata.FieldSpec,
	src metadata.Source,
	valuePtr *string,
) []huh.Field {
	var inputs []huh.Field

	// Create appropriate input based on spec.InputType.
	switch spec.InputType {
	case metadata.InputTypeTextArea:
		inputs = append(inputs, ie.createTextAreaInput(spec.Key, spec.Required, spec.Weight, spec.Placeholder, spec.Suggestions, spec.Sources, src, valuePtr))
	case metadata.InputTypeSelect:
		inputs = append(inputs, ie.createSelectInput(spec.Key, spec.Required, spec.Weight, spec.Placeholder, spec.Suggestions, spec.Sources, src, valuePtr))
	case metadata.InputTypeMultiText:
		inputs = append(inputs, ie.createMultiTextInput(spec.Key, spec.Required, spec.Weight, spec.Placeholder, spec.Suggestions, spec.Sources, src, valuePtr))
	default: // InputTypeText
		inputs = append(inputs, ie.createTextInput(spec.Key, spec.Required, spec.Weight, spec.Placeholder, spec.Suggestions, spec.Sources, src, valuePtr))
	}

	return inputs
}

// createTextInput creates a standard text input field.
func (ie *InteractiveEnricher) createTextInput(
	key metadata.Key,
	required bool,
	weight float64,
	placeholder string,
	suggestions []string,
	sources []func(metadata.Source) (any, bool),
	src metadata.Source,
	valuePtr *string,
) huh.Field {
	// Get suggestions from sources if not explicitly provided.
	if len(suggestions) == 0 {
		suggestions = ie.getSuggestionsFromSources(sources, src)
	}

	description := ie.formatDescription(suggestions, placeholder, false)
	title := ie.formatTitle(key, weight, required)

	input := huh.NewInput().
		Title(title).
		Description(description).
		Placeholder(placeholder).
		Value(valuePtr).
		Validate(func(s string) error {
			if required && strings.TrimSpace(s) == "" {
				return fmt.Errorf("this field is required")
			}
			return nil
		})

	return input
}

// createMultiTextInput creates input for comma-separated arrays.
func (ie *InteractiveEnricher) createMultiTextInput(
	key metadata.Key,
	required bool,
	weight float64,
	placeholder string,
	suggestions []string,
	sources []func(metadata.Source) (any, bool),
	src metadata.Source,
	valuePtr *string,
) huh.Field {
	if len(suggestions) == 0 {
		suggestions = ie.getSuggestionsFromSources(sources, src)
	}

	description := ie.formatDescription(suggestions, placeholder, true)
	title := ie.formatTitle(key, weight, required)

	input := huh.NewInput().
		Title(title).
		Description(description).
		Placeholder(placeholder).
		Value(valuePtr).
		Validate(func(s string) error {
			if required && strings.TrimSpace(s) == "" {
				return fmt.Errorf("this field is required")
			}
			return nil
		})

	return input
}

// createTextAreaInput creates a multi-line text input.
func (ie *InteractiveEnricher) createTextAreaInput(
	key metadata.Key,
	required bool,
	weight float64,
	placeholder string,
	suggestions []string,
	sources []func(metadata.Source) (any, bool),
	src metadata.Source,
	valuePtr *string,
) huh.Field {
	if len(suggestions) == 0 {
		suggestions = ie.getSuggestionsFromSources(sources, src)
	}

	title := ie.formatTitle(key, weight, required)
	description := ie.formatDescription(suggestions, placeholder, false)

	input := huh.NewText().
		Title(title).
		Description(description).
		Placeholder(placeholder).
		Value(valuePtr).
		Lines(5).
		CharLimit(1000).
		Validate(func(s string) error {
			if required && strings.TrimSpace(s) == "" {
				return fmt.Errorf("this field is required")
			}
			return nil
		})

	return input
}

// createSelectInput creates a select input with predefined options.
func (ie *InteractiveEnricher) createSelectInput(
	key metadata.Key,
	required bool,
	weight float64,
	placeholder string,
	suggestions []string,
	sources []func(metadata.Source) (any, bool),
	src metadata.Source,
	valuePtr *string,
) huh.Field {
	if len(suggestions) == 0 {
		suggestions = ie.getSuggestionsFromSources(sources, src)
	}

	title := ie.formatTitle(key, weight, required)

	options := []huh.Option[string]{}
	for _, suggestion := range suggestions {
		options = append(options, huh.NewOption(suggestion, suggestion))
	}

	input := huh.NewSelect[string]().
		Title(title).
		Description(placeholder).
		Options(options...).
		Value(valuePtr).
		Validate(func(s string) error {
			if required && s == "" {
				return fmt.Errorf("this field is required")
			}
			return nil
		})

	return input
}

// Helper methods.

func (ie *InteractiveEnricher) formatTitle(key metadata.Key, weight float64, required bool) string {
	requiredLabel := ""
	if required {
		requiredLabel = ui.Error.Render(" [REQUIRED]")
	}

	weightLabel := ui.Muted.Render(fmt.Sprintf(" (weight: %.1f)", weight))

	// Simplify the key for display.
	displayKey := ie.simplifyKeyForDisplay(key)

	return fmt.Sprintf("%s%s%s", displayKey, weightLabel, requiredLabel)
}

func (ie *InteractiveEnricher) formatDescription(suggestions []string, placeholder string, isArray bool) string {
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

func (ie *InteractiveEnricher) getSuggestionsFromSources(sources []func(metadata.Source) (any, bool), src metadata.Source) []string {
	// Try to get value from sources.
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

func (ie *InteractiveEnricher) simplifyKeyForDisplay(key metadata.Key) string {
	// we could define extra logic here to transform key in more readable form.
	// for now, just return the string representation for clarity.
	return string(key)
}

func (ie *InteractiveEnricher) camelToTitle(s string) string {
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

// EnrichDatasetInteractive enriches dataset fields using interactive forms.
func (ie *InteractiveEnricher) EnrichDatasetInteractive(
	comp *cdx.Component,
	missingFields []metadata.DatasetFieldSpec,
	src metadata.DatasetSource,
	tgt metadata.DatasetTarget,
) (map[metadata.DatasetKey]string, error) {
	if len(missingFields) == 0 {
		return nil, nil
	}

	// Storage for form values - use map of pointers.
	valueStore := make(map[metadata.DatasetKey]*string)
	for _, spec := range missingFields {
		val := ""
		valueStore[spec.Key] = &val
	}

	// Create form groups.
	formGroups := []*huh.Group{}

	// Add intro note.
	formGroups = append(formGroups, huh.NewGroup(
		huh.NewNote().
			Title(fmt.Sprintf("Dataset Enrichment: %s", comp.Name)).
			Description("Please provide values for the missing dataset fields.\nPress Enter to skip optional fields.").
			Next(true).
			NextLabel("Continue"),
	))

	// Create inputs for each field.
	for _, spec := range missingFields {
		fieldInputs := ie.createDatasetFieldInput(spec, src, valueStore[spec.Key])
		if len(fieldInputs) > 0 {
			formGroups = append(formGroups, huh.NewGroup(fieldInputs...))
		}
	}

	// Skip if no inputs were created.
	if len(formGroups) <= 1 {
		return nil, nil
	}

	// Create and run form.
	form := huh.NewForm(formGroups...)
	err := form.Run()
	if err != nil {
		return nil, err
	}

	// Now read the values from the pointers and apply them.
	changes := make(map[metadata.DatasetKey]string)
	for _, spec := range missingFields {
		strValue := *valueStore[spec.Key]
		if strValue == "" {
			continue
		}

		// Apply the value.
		err := metadata.ApplyDatasetUserValue(spec, strValue, tgt)
		if err != nil {
			// Continue on error, just skip this field.
			continue
		}
		changes[spec.Key] = strValue
	}

	return changes, nil
}

// createDatasetFieldInput creates form inputs for a dataset field spec.
func (ie *InteractiveEnricher) createDatasetFieldInput(
	spec metadata.DatasetFieldSpec,
	src metadata.DatasetSource,
	valuePtr *string,
) []huh.Field {
	var inputs []huh.Field

	// Create appropriate input based on spec.InputType.
	switch spec.InputType {
	case metadata.InputTypeTextArea:
		inputs = append(inputs, ie.createDatasetTextAreaInput(spec.Key, spec.Required, spec.Weight, spec.Placeholder, spec.Suggestions, spec.Sources, src, valuePtr))
	case metadata.InputTypeSelect:
		inputs = append(inputs, ie.createDatasetSelectInput(spec.Key, spec.Required, spec.Weight, spec.Placeholder, spec.Suggestions, spec.Sources, src, valuePtr))
	case metadata.InputTypeMultiText:
		inputs = append(inputs, ie.createDatasetMultiTextInput(spec.Key, spec.Required, spec.Weight, spec.Placeholder, spec.Suggestions, spec.Sources, src, valuePtr))
	default: // InputTypeText
		inputs = append(inputs, ie.createDatasetTextInput(spec.Key, spec.Required, spec.Weight, spec.Placeholder, spec.Suggestions, spec.Sources, src, valuePtr))
	}

	return inputs
}

// Dataset-specific input creators.
func (ie *InteractiveEnricher) createDatasetTextInput(
	key metadata.DatasetKey,
	required bool,
	weight float64,
	placeholder string,
	suggestions []string,
	sources []func(metadata.DatasetSource) (any, bool),
	src metadata.DatasetSource,
	valuePtr *string,
) huh.Field {
	if len(suggestions) == 0 {
		suggestions = ie.getDatasetSuggestionsFromSources(sources, src)
	}

	title := ie.formatDatasetTitle(key, weight, required)
	description := ie.formatDescription(suggestions, placeholder, false)

	input := huh.NewInput().
		Title(title).
		Description(description).
		Placeholder(placeholder).
		Value(valuePtr).
		Validate(func(s string) error {
			if required && strings.TrimSpace(s) == "" {
				return fmt.Errorf("this field is required")
			}
			return nil
		})

	return input
}

func (ie *InteractiveEnricher) createDatasetMultiTextInput(
	key metadata.DatasetKey,
	required bool,
	weight float64,
	placeholder string,
	suggestions []string,
	sources []func(metadata.DatasetSource) (any, bool),
	src metadata.DatasetSource,
	valuePtr *string,
) huh.Field {
	if len(suggestions) == 0 {
		suggestions = ie.getDatasetSuggestionsFromSources(sources, src)
	}

	title := ie.formatDatasetTitle(key, weight, required)
	description := ie.formatDescription(suggestions, placeholder, true)

	input := huh.NewInput().
		Title(title).
		Description(description).
		Placeholder(placeholder).
		Value(valuePtr).
		Validate(func(s string) error {
			if required && strings.TrimSpace(s) == "" {
				return fmt.Errorf("this field is required")
			}
			return nil
		})

	return input
}

func (ie *InteractiveEnricher) createDatasetTextAreaInput(
	key metadata.DatasetKey,
	required bool,
	weight float64,
	placeholder string,
	suggestions []string,
	sources []func(metadata.DatasetSource) (any, bool),
	src metadata.DatasetSource,
	valuePtr *string,
) huh.Field {
	if len(suggestions) == 0 {
		suggestions = ie.getDatasetSuggestionsFromSources(sources, src)
	}

	title := ie.formatDatasetTitle(key, weight, required)
	description := ie.formatDescription(suggestions, placeholder, false)

	input := huh.NewText().
		Title(title).
		Description(description).
		Placeholder(placeholder).
		Value(valuePtr).
		Lines(5).
		CharLimit(1000)

	return input
}

func (ie *InteractiveEnricher) createDatasetSelectInput(
	key metadata.DatasetKey,
	required bool,
	weight float64,
	placeholder string,
	suggestions []string,
	sources []func(metadata.DatasetSource) (any, bool),
	src metadata.DatasetSource,
	valuePtr *string,
) huh.Field {
	if len(suggestions) == 0 {
		suggestions = ie.getDatasetSuggestionsFromSources(sources, src)
	}

	title := ie.formatDatasetTitle(key, weight, required)

	options := []huh.Option[string]{}
	for _, suggestion := range suggestions {
		options = append(options, huh.NewOption(suggestion, suggestion))
	}

	input := huh.NewSelect[string]().
		Title(title).
		Description(placeholder).
		Options(options...).
		Value(valuePtr).
		Validate(func(s string) error {
			if required && s == "" {
				return fmt.Errorf("this field is required")
			}
			return nil
		})

	return input
}

func (ie *InteractiveEnricher) formatDatasetTitle(key metadata.DatasetKey, weight float64, required bool) string {
	requiredLabel := ""
	if required {
		requiredLabel = ui.Error.Render(" [REQUIRED]")
	}

	weightLabel := ui.Muted.Render(fmt.Sprintf(" (weight: %.1f)", weight))

	displayKey := ie.simplifyDatasetKeyForDisplay(key)

	return fmt.Sprintf("%s%s%s", displayKey, weightLabel, requiredLabel)
}

func (ie *InteractiveEnricher) simplifyDatasetKeyForDisplay(key metadata.DatasetKey) string {
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

	return ie.camelToTitle(lastPart)
}

func (ie *InteractiveEnricher) getDatasetSuggestionsFromSources(sources []func(metadata.DatasetSource) (any, bool), src metadata.DatasetSource) []string {
	// Try to get value from sources.
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
