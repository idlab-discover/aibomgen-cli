package metadata

import (
	"fmt"

	cdx "github.com/CycloneDX/cyclonedx-go"
)

type applyInput struct {
	Value any
	Force bool
}

// componentApply mutates a non-nil component; baseURL is the target's HuggingFaceBaseURL.
type componentApply func(c *cdx.Component, in applyInput, baseURL string) error

// onComponent guards a componentApply against a nil target component.
func onComponent(fn componentApply) func(Target, applyInput) error {
	return func(tgt Target, in applyInput) error {
		if tgt.Component == nil {
			return fmt.Errorf("component is nil")
		}
		return fn(tgt.Component, in, tgt.HuggingFaceBaseURL)
	}
}

// onDatasetComponent is onComponent for dataset targets.
func onDatasetComponent(fn componentApply) func(DatasetTarget, applyInput) error {
	return func(tgt DatasetTarget, in applyInput) error {
		if tgt.Component == nil {
			return fmt.Errorf("component is nil")
		}
		return fn(tgt.Component, in, tgt.HuggingFaceBaseURL)
	}
}

// onModelCard guards an Apply func against a nil target model card.
func onModelCard(fn func(*cdx.MLModelCard, applyInput) error) func(Target, applyInput) error {
	return func(tgt Target, in applyInput) error {
		if tgt.ModelCard == nil {
			return fmt.Errorf("modelCard is nil")
		}
		return fn(tgt.ModelCard, in)
	}
}

// applyFirst applies the first source value that apply accepts.
// A failed apply (e.g. a placeholder value) falls through to the next source.
func applyFirst[S, T any](sources []func(S) (any, bool), apply func(T, applyInput) error, src S, tgt T) {
	if apply == nil {
		return
	}
	for _, get := range sources {
		if get == nil {
			continue
		}
		if value, ok := get(src); ok && apply(tgt, applyInput{Value: value}) == nil {
			return
		}
	}
}

// applyParsed parses a user value and applies it with Force set.
func applyParsed[T any](key fmt.Stringer, parse func(string) (any, error), apply func(T, applyInput) error, value string, tgt T) error {
	if parse == nil || apply == nil {
		return fmt.Errorf("spec missing Parse/Apply for %s", key)
	}
	parsed, err := parse(value)
	if err != nil {
		return err
	}
	return apply(tgt, applyInput{Value: parsed, Force: true})
}

// ApplyFromSources applies the first source value that spec.Apply accepts.
func ApplyFromSources(spec FieldSpec, src Source, tgt Target) {
	applyFirst(spec.Sources, spec.Apply, src, tgt)
}

// ApplyUserValue parses and applies a user-provided value using spec.Parse and spec.Apply.
func ApplyUserValue(spec FieldSpec, value string, tgt Target) error {
	return applyParsed(spec.Key, spec.Parse, spec.Apply, value, tgt)
}

// ApplyDatasetFromSources applies the first dataset source value that spec.Apply accepts.
func ApplyDatasetFromSources(spec DatasetFieldSpec, src DatasetSource, tgt DatasetTarget) {
	applyFirst(spec.Sources, spec.Apply, src, tgt)
}

// ApplyDatasetUserValue parses and applies a dataset user value.
func ApplyDatasetUserValue(spec DatasetFieldSpec, value string, tgt DatasetTarget) error {
	return applyParsed(spec.Key, spec.Parse, spec.Apply, value, tgt)
}
