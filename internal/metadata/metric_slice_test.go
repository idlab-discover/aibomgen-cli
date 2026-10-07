package metadata

import (
	"testing"

	"github.com/idlab-discover/aibomgen-cli/internal/fetcher"
)

func TestMetricSlice(t *testing.T) {
	tests := []struct{ dataset, split, want string }{
		{"MTEB AmazonCounterfactualClassification (en)", "test", "MTEB AmazonCounterfactualClassification (en) / test"},
		{"glue", "", "glue"},
		{"", "validation", "validation"},
		{" ", " ", ""},
	}
	for _, tt := range tests {
		if got := metricSlice(tt.dataset, tt.split); got != tt.want {
			t.Errorf("metricSlice(%q, %q) = %q, want %q", tt.dataset, tt.split, got, tt.want)
		}
	}
}

func TestPerformanceMetricsSlice(t *testing.T) {
	comp := applyModel(Source{ModelID: "org/m", Readme: &fetcher.ModelReadmeCard{
		ModelIndexMetrics: []fetcher.ModelIndexMetric{
			{Type: "accuracy", Value: "73.8", Dataset: "MTEB A (en)", Split: "test"},
			{Type: "accuracy", Value: "61.2", Dataset: "MTEB B", Split: "test"},
		},
		Metrics: []string{"accuracy", "f1"},
	}})
	qa := comp.ModelCard.QuantitativeAnalysis
	if qa == nil || qa.PerformanceMetrics == nil {
		t.Fatalf("no performance metrics")
	}
	got := *qa.PerformanceMetrics
	if len(got) != 3 {
		t.Fatalf("metrics = %+v, want two sliced accuracy metrics plus front matter f1", got)
	}
	if got[0].Slice != "MTEB A (en) / test" || got[1].Slice != "MTEB B / test" {
		t.Fatalf("slices = %q, %q", got[0].Slice, got[1].Slice)
	}
	if got[2].Type != "f1" || got[2].Slice != "" {
		t.Fatalf("front matter metric = %+v", got[2])
	}
}
