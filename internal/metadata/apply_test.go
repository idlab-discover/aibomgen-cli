package metadata

import (
	"errors"
	"testing"
)

func TestApplyFromSources_FallsBackWhenApplyFails(t *testing.T) {
	var applied any
	spec := FieldSpec{
		Key: "test",
		Sources: []func(Source) (any, bool){
			func(Source) (any, bool) { return "placeholder", true },
			func(Source) (any, bool) { return "real", true },
		},
		Apply: func(_ Target, in applyInput) error {
			if in.Value == "placeholder" {
				return errors.New("placeholder")
			}
			applied = in.Value
			return nil
		},
	}
	ApplyFromSources(spec, Source{}, Target{})
	if applied != "real" {
		t.Fatalf("applied = %v, want fallback source value %q", applied, "real")
	}
}
