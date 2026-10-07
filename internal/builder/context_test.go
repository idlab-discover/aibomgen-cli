package builder

import (
	"reflect"
	"testing"
)

func TestDefaultOptions(t *testing.T) {
	tests := []struct {
		name string
		want Options
	}{
		{name: "defaults", want: Options{HuggingFaceBaseURL: "https://huggingface.co/"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := DefaultOptions(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("DefaultOptions() = %v, want %v", got, tt.want)
			}
		})
	}
}
