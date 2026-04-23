package generator

import (
	"context"
	"net/http"
	"reflect"
	"testing"
	"time"

	cdx "github.com/CycloneDX/cyclonedx-go"
	"github.com/idlab-discover/aibomgen-cli/internal/builder"
	"github.com/idlab-discover/aibomgen-cli/internal/fetcher"
	"github.com/idlab-discover/aibomgen-cli/pkg/aibomgen/scanner"
)

// Mock BOM Builder for testing.
type mockBOMBuilder struct {
	buildFunc        func(builder.BuildContext) (*cdx.BOM, error)
	buildDatasetFunc func(builder.DatasetBuildContext) (*cdx.Component, error)
}

func (m *mockBOMBuilder) Build(ctx builder.BuildContext) (*cdx.BOM, error) {
	if m.buildFunc != nil {
		return m.buildFunc(ctx)
	}
	return &cdx.BOM{}, nil
}

func (m *mockBOMBuilder) BuildDataset(ctx builder.DatasetBuildContext) (*cdx.Component, error) {
	if m.buildDatasetFunc != nil {
		return m.buildDatasetFunc(ctx)
	}
	return &cdx.Component{Name: ctx.DatasetID}, nil
}

// Mock Fetchers for testing.
type mockModelAPIFetcher struct {
	fetchFunc func(string) (*fetcher.ModelAPIResponse, error)
}

func (m *mockModelAPIFetcher) Fetch(id string) (*fetcher.ModelAPIResponse, error) {
	if m.fetchFunc != nil {
		return m.fetchFunc(id)
	}
	return &fetcher.ModelAPIResponse{}, nil
}

type mockModelReadmeFetcher struct {
	fetchFunc func(string) (*fetcher.ModelReadmeCard, error)
}

func (m *mockModelReadmeFetcher) Fetch(id string) (*fetcher.ModelReadmeCard, error) {
	if m.fetchFunc != nil {
		return m.fetchFunc(id)
	}
	return &fetcher.ModelReadmeCard{}, nil
}

type mockDatasetAPIFetcher struct {
	fetchFunc func(string) (*fetcher.DatasetAPIResponse, error)
}

func (m *mockDatasetAPIFetcher) Fetch(id string) (*fetcher.DatasetAPIResponse, error) {
	if m.fetchFunc != nil {
		return m.fetchFunc(id)
	}
	return &fetcher.DatasetAPIResponse{}, nil
}

type mockDatasetReadmeFetcher struct {
	fetchFunc func(string) (*fetcher.DatasetReadmeCard, error)
}

func (m *mockDatasetReadmeFetcher) Fetch(id string) (*fetcher.DatasetReadmeCard, error) {
	if m.fetchFunc != nil {
		return m.fetchFunc(id)
	}
	return &fetcher.DatasetReadmeCard{}, nil
}

func TestBuildDummyBOM(t *testing.T) {
	// Save originals.
	originalDummyFetcherSet := newDummyFetcherSet
	originalBuilder := newBOMBuilder

	tests := []struct {
		name    string
		setup   func()
		wantErr bool
		check   func(*testing.T, []DiscoveredBOM)
	}{
		{
			name:    "builds dummy BOM successfully",
			setup:   func() {},
			wantErr: false,
			check: func(t *testing.T, got []DiscoveredBOM) {
				if len(got) != 1 {
					t.Errorf("Expected 1 BOM, got %d", len(got))
					return
				}
				if got[0].BOM == nil {
					t.Error("BOM is nil")
					return
				}
				if got[0].Discovery.ID != "dummy-org/dummy-model" {
					t.Errorf("Expected discovery ID 'dummy-org/dummy-model', got %q", got[0].Discovery.ID)
				}
				// Should have datasets from dummy data.
				if got[0].BOM.Components != nil && len(*got[0].BOM.Components) > 0 {
					t.Logf("BOM has %d dataset components", len(*got[0].BOM.Components))
				}
			},
		},
		{
			name: "handles model API fetch error",
			setup: func() {
				newDummyFetcherSet = func() fetcherSet {
					return fetcherSet{
						modelAPI: &mockModelAPIFetcher{
							fetchFunc: func(id string) (*fetcher.ModelAPIResponse, error) {
								return nil, context.Canceled
							},
						},
						modelReadme:   &fetcher.DummyModelReadmeFetcher{},
						datasetAPI:    &fetcher.DummyDatasetAPIFetcher{},
						datasetReadme: &fetcher.DummyDatasetReadmeFetcher{},
					}
				}
			},
			wantErr: true,
		},
		{
			name: "handles model README fetch error",
			setup: func() {
				newDummyFetcherSet = func() fetcherSet {
					return fetcherSet{
						modelAPI: &fetcher.DummyModelAPIFetcher{},
						modelReadme: &mockModelReadmeFetcher{
							fetchFunc: func(id string) (*fetcher.ModelReadmeCard, error) {
								return nil, context.Canceled
							},
						},
						datasetAPI:    &fetcher.DummyDatasetAPIFetcher{},
						datasetReadme: &fetcher.DummyDatasetReadmeFetcher{},
					}
				}
			},
			wantErr: true,
		},
		{
			name: "handles BOM build error",
			setup: func() {
				newBOMBuilder = func() bomBuilder {
					return &mockBOMBuilder{
						buildFunc: func(ctx builder.BuildContext) (*cdx.BOM, error) {
							return nil, context.Canceled
						},
					}
				}
			},
			wantErr: true,
		},
		{
			name: "handles dataset API fetch errors gracefully",
			setup: func() {
				newDummyFetcherSet = func() fetcherSet {
					return fetcherSet{
						modelAPI:    &fetcher.DummyModelAPIFetcher{},
						modelReadme: &fetcher.DummyModelReadmeFetcher{},
						datasetAPI: &mockDatasetAPIFetcher{
							fetchFunc: func(id string) (*fetcher.DatasetAPIResponse, error) {
								return nil, context.Canceled // Dataset fetch fails
							},
						},
						datasetReadme: &fetcher.DummyDatasetReadmeFetcher{},
					}
				}
			},
			wantErr: false,
			check: func(t *testing.T, got []DiscoveredBOM) {
				if len(got) != 1 {
					t.Errorf("Expected 1 BOM, got %d", len(got))
					return
				}
				// Dataset fetch failed, should have no dataset components.
			},
		},
		{
			name: "handles dataset readme fetch error in BuildDummyBOM",
			setup: func() {
				newDummyFetcherSet = func() fetcherSet {
					return fetcherSet{
						modelAPI:    &fetcher.DummyModelAPIFetcher{},
						modelReadme: &fetcher.DummyModelReadmeFetcher{},
						datasetAPI:  &fetcher.DummyDatasetAPIFetcher{},
						datasetReadme: &mockDatasetReadmeFetcher{
							fetchFunc: func(id string) (*fetcher.DatasetReadmeCard, error) {
								return nil, context.Canceled
							},
						},
					}
				}
			},
			wantErr: false,
			check: func(t *testing.T, got []DiscoveredBOM) {
				if len(got) != 1 {
					t.Errorf("Expected 1 BOM, got %d", len(got))
					return
				}
				// Dataset readme fetch failed but should still have dataset component (from API data).
				if got[0].BOM.Components == nil || len(*got[0].BOM.Components) == 0 {
					t.Error("Expected dataset components despite readme failure")
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Restore originals before each test case.
			newDummyFetcherSet = originalDummyFetcherSet
			newBOMBuilder = originalBuilder

			if tt.setup != nil {
				tt.setup()
			}
			got, err := BuildDummyBOM()
			if (err != nil) != tt.wantErr {
				t.Errorf("BuildDummyBOM() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.check != nil && !tt.wantErr {
				tt.check(t, got)
			}
		})
	}
}

func TestBuildPerDiscovery(t *testing.T) {
	// Save originals and restore after each test.
	originalBuilder := newBOMBuilder
	originalFetcherSet := newFetcherSet
	defer func() {
		newBOMBuilder = originalBuilder
		newFetcherSet = originalFetcherSet
	}()

	type args struct {
		discoveries []scanner.Discovery
		opts        GenerateOptions
	}
	tests := []struct {
		name    string
		args    args
		setup   func()
		wantErr bool
		check   func(*testing.T, []DiscoveredBOM)
	}{
		{
			name: "builds BOM for single discovery",
			args: args{
				discoveries: []scanner.Discovery{
					{ID: "test-model", Name: "test-model", Type: "huggingface"},
				},
				opts: GenerateOptions{Timeout: 1 * time.Second},
			},
			setup: func() {
				newBOMBuilder = func() bomBuilder {
					return &mockBOMBuilder{
						buildFunc: func(bctx builder.BuildContext) (*cdx.BOM, error) {
							return &cdx.BOM{SerialNumber: "test-serial"}, nil
						},
					}
				}
			},
			wantErr: false,
			check: func(t *testing.T, got []DiscoveredBOM) {
				if len(got) != 1 {
					t.Errorf("Expected 1 BOM, got %d", len(got))
				}
			},
		},
		{
			name: "builds BOM for empty discovery list",
			args: args{
				discoveries: []scanner.Discovery{},
				opts:        GenerateOptions{Timeout: 1 * time.Second},
			},
			setup: func() {
				newBOMBuilder = func() bomBuilder { return &mockBOMBuilder{} }
			},
			wantErr: false,
			check: func(t *testing.T, got []DiscoveredBOM) {
				if len(got) != 0 {
					t.Errorf("Expected 0 BOMs, got %d", len(got))
				}
			},
		},
		{
			name: "uses default timeout when zero",
			args: args{
				discoveries: []scanner.Discovery{
					{ID: "test-model", Name: "test-model", Type: "huggingface"},
				},
				opts: GenerateOptions{Timeout: 0}, // Zero timeout should use default
			},
			setup: func() {
				newBOMBuilder = func() bomBuilder {
					return &mockBOMBuilder{
						buildFunc: func(bctx builder.BuildContext) (*cdx.BOM, error) {
							return &cdx.BOM{}, nil
						},
					}
				}
			},
			wantErr: false,
			check: func(t *testing.T, got []DiscoveredBOM) {
				if len(got) != 1 {
					t.Errorf("Expected 1 BOM, got %d", len(got))
				}
			},
		},
		{
			name: "uses Name when ID is empty",
			args: args{
				discoveries: []scanner.Discovery{
					{ID: "", Name: "fallback-name", Type: "huggingface"},
				},
				opts: GenerateOptions{Timeout: 1 * time.Second},
			},
			setup: func() {
				newBOMBuilder = func() bomBuilder {
					return &mockBOMBuilder{
						buildFunc: func(bctx builder.BuildContext) (*cdx.BOM, error) {
							return &cdx.BOM{}, nil
						},
					}
				}
			},
			wantErr: false,
			check: func(t *testing.T, got []DiscoveredBOM) {
				if len(got) != 1 {
					t.Errorf("Expected 1 BOM, got %d", len(got))
				}
			},
		},
		{
			name: "calls progress callback during build",
			args: args{
				discoveries: []scanner.Discovery{
					{ID: "test-model", Name: "test-model", Type: "huggingface"},
				},
				opts: GenerateOptions{
					Timeout:    1 * time.Second,
					OnProgress: func(event ProgressEvent) {},
				},
			},
			setup: func() {
				newBOMBuilder = func() bomBuilder {
					return &mockBOMBuilder{
						buildFunc: func(bctx builder.BuildContext) (*cdx.BOM, error) {
							return &cdx.BOM{SerialNumber: "test-serial"}, nil
						},
					}
				}
			},
			wantErr: false,
			check: func(t *testing.T, got []DiscoveredBOM) {
				if len(got) != 1 {
					t.Errorf("Expected 1 BOM, got %d", len(got))
				}
			},
		},
		{
			name: "skips model when BOM build fails",
			args: args{
				discoveries: []scanner.Discovery{
					{ID: "test-model", Name: "test-model", Type: "huggingface"},
				},
				opts: GenerateOptions{
					Timeout:    1 * time.Second,
					OnProgress: func(event ProgressEvent) {},
				},
			},
			setup: func() {
				newBOMBuilder = func() bomBuilder {
					return &mockBOMBuilder{
						buildFunc: func(bctx builder.BuildContext) (*cdx.BOM, error) {
							return nil, context.Canceled
						},
					}
				}
			},
			wantErr: false,
			check: func(t *testing.T, got []DiscoveredBOM) {
				if len(got) != 0 {
					t.Errorf("Expected 0 BOMs (model skipped on build error), got %d", len(got))
				}
			},
		},
		{
			name: "builds datasets from model metadata",
			args: args{
				discoveries: []scanner.Discovery{
					{ID: "model-with-datasets", Name: "model-with-datasets", Type: "huggingface"},
				},
				opts: GenerateOptions{Timeout: 1 * time.Second},
			},
			setup: func() {
				newBOMBuilder = func() bomBuilder {
					return &mockBOMBuilder{
						buildFunc: func(bctx builder.BuildContext) (*cdx.BOM, error) {
							return &cdx.BOM{}, nil
						},
						buildDatasetFunc: func(bctx builder.DatasetBuildContext) (*cdx.Component, error) {
							return &cdx.Component{Type: cdx.ComponentTypeData, Name: bctx.DatasetID}, nil
						},
					}
				}
				newFetcherSet = func(httpClient *http.Client) fetcherSet {
					return fetcherSet{
						modelAPI: &mockModelAPIFetcher{
							fetchFunc: func(id string) (*fetcher.ModelAPIResponse, error) {
								return &fetcher.ModelAPIResponse{
									CardData: map[string]interface{}{"datasets": []interface{}{"dataset-1"}},
								}, nil
							},
						},
						modelReadme: &mockModelReadmeFetcher{
							fetchFunc: func(id string) (*fetcher.ModelReadmeCard, error) {
								return &fetcher.ModelReadmeCard{}, nil
							},
						},
						datasetAPI: &mockDatasetAPIFetcher{
							fetchFunc: func(id string) (*fetcher.DatasetAPIResponse, error) {
								return &fetcher.DatasetAPIResponse{ID: id}, nil
							},
						},
						datasetReadme: &mockDatasetReadmeFetcher{
							fetchFunc: func(id string) (*fetcher.DatasetReadmeCard, error) {
								return &fetcher.DatasetReadmeCard{}, nil
							},
						},
					}
				}
			},
			wantErr: false,
			check: func(t *testing.T, got []DiscoveredBOM) {
				if len(got) != 1 {
					t.Errorf("Expected 1 BOM, got %d", len(got))
					return
				}
				if got[0].BOM.Components == nil || len(*got[0].BOM.Components) != 1 {
					t.Errorf("Expected 1 dataset component, got %v", got[0].BOM.Components)
				}
			},
		},
		{
			name: "skips datasets that fail to fetch",
			args: args{
				discoveries: []scanner.Discovery{
					{ID: "model-with-api", Name: "model-with-api", Type: "huggingface"},
				},
				opts: GenerateOptions{
					HFToken:    "test-token",
					Timeout:    1 * time.Second,
					OnProgress: func(event ProgressEvent) {},
				},
			},
			setup: func() {
				newBOMBuilder = func() bomBuilder {
					return &mockBOMBuilder{
						buildFunc: func(bctx builder.BuildContext) (*cdx.BOM, error) {
							return &cdx.BOM{}, nil
						},
						buildDatasetFunc: func(bctx builder.DatasetBuildContext) (*cdx.Component, error) {
							return &cdx.Component{Name: bctx.DatasetID}, nil
						},
					}
				}
				newFetcherSet = func(httpClient *http.Client) fetcherSet {
					return fetcherSet{
						modelAPI: &mockModelAPIFetcher{
							fetchFunc: func(id string) (*fetcher.ModelAPIResponse, error) {
								return &fetcher.ModelAPIResponse{
									ID: id,
									CardData: map[string]interface{}{
										"datasets": []interface{}{"test-dataset", "failing-dataset"},
									},
								}, nil
							},
						},
						modelReadme: &mockModelReadmeFetcher{
							fetchFunc: func(id string) (*fetcher.ModelReadmeCard, error) {
								return &fetcher.ModelReadmeCard{
									Datasets: []string{"readme-dataset"},
								}, nil
							},
						},
						datasetAPI: &mockDatasetAPIFetcher{
							fetchFunc: func(id string) (*fetcher.DatasetAPIResponse, error) {
								if id == "failing-dataset" {
									return nil, context.Canceled
								}
								return &fetcher.DatasetAPIResponse{ID: id}, nil
							},
						},
						datasetReadme: &mockDatasetReadmeFetcher{
							fetchFunc: func(id string) (*fetcher.DatasetReadmeCard, error) {
								return &fetcher.DatasetReadmeCard{}, nil
							},
						},
					}
				}
			},
			wantErr: false,
			check: func(t *testing.T, got []DiscoveredBOM) {
				if len(got) != 1 {
					t.Errorf("Expected 1 BOM, got %d", len(got))
					return
				}
				// 2 datasets succeed (test-dataset, readme-dataset); failing-dataset is skipped.
				if got[0].BOM.Components == nil || len(*got[0].BOM.Components) != 2 {
					t.Errorf("Expected 2 dataset components (1 failed), got %v", got[0].BOM.Components)
				}
			},
		},
		{
			name: "handles dataset build errors gracefully",
			args: args{
				discoveries: []scanner.Discovery{
					{ID: "model-with-failing-dataset", Name: "model-with-failing-dataset", Type: "huggingface"},
				},
				opts: GenerateOptions{HFToken: "test-token", Timeout: 1 * time.Second},
			},
			setup: func() {
				newBOMBuilder = func() bomBuilder {
					return &mockBOMBuilder{
						buildFunc: func(bctx builder.BuildContext) (*cdx.BOM, error) {
							return &cdx.BOM{}, nil
						},
						buildDatasetFunc: func(bctx builder.DatasetBuildContext) (*cdx.Component, error) {
							return nil, context.Canceled
						},
					}
				}
				newFetcherSet = func(httpClient *http.Client) fetcherSet {
					return fetcherSet{
						modelAPI: &mockModelAPIFetcher{
							fetchFunc: func(id string) (*fetcher.ModelAPIResponse, error) {
								return &fetcher.ModelAPIResponse{
									CardData: map[string]interface{}{"datasets": []interface{}{"failing-dataset"}},
								}, nil
							},
						},
						modelReadme: &mockModelReadmeFetcher{},
						datasetAPI: &mockDatasetAPIFetcher{
							fetchFunc: func(id string) (*fetcher.DatasetAPIResponse, error) {
								return &fetcher.DatasetAPIResponse{ID: id}, nil
							},
						},
						datasetReadme: &mockDatasetReadmeFetcher{},
					}
				}
			},
			wantErr: false,
			check: func(t *testing.T, got []DiscoveredBOM) {
				if len(got) != 1 {
					t.Errorf("Expected 1 BOM, got %d", len(got))
					return
				}
				if got[0].BOM.Components != nil && len(*got[0].BOM.Components) > 0 {
					t.Errorf("Expected no dataset components due to build error, got %d", len(*got[0].BOM.Components))
				}
			},
		},
		{
			name: "handles model API and README fetch errors gracefully",
			args: args{
				discoveries: []scanner.Discovery{
					{ID: "model-with-fetch-errors", Name: "model-with-fetch-errors", Type: "huggingface"},
				},
				opts: GenerateOptions{HFToken: "test-token", Timeout: 1 * time.Second},
			},
			setup: func() {
				newBOMBuilder = func() bomBuilder {
					return &mockBOMBuilder{
						buildFunc: func(bctx builder.BuildContext) (*cdx.BOM, error) {
							return &cdx.BOM{}, nil
						},
					}
				}
				newFetcherSet = func(httpClient *http.Client) fetcherSet {
					return fetcherSet{
						modelAPI: &mockModelAPIFetcher{
							fetchFunc: func(id string) (*fetcher.ModelAPIResponse, error) {
								return nil, context.Canceled
							},
						},
						modelReadme: &mockModelReadmeFetcher{
							fetchFunc: func(id string) (*fetcher.ModelReadmeCard, error) {
								return nil, context.Canceled
							},
						},
						datasetAPI:    &mockDatasetAPIFetcher{},
						datasetReadme: &mockDatasetReadmeFetcher{},
					}
				}
			},
			wantErr: false,
			check: func(t *testing.T, got []DiscoveredBOM) {
				if len(got) != 1 {
					t.Errorf("Expected 1 BOM, got %d", len(got))
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setup != nil {
				tt.setup()
			}
			got, err := BuildPerDiscovery(tt.args.discoveries, tt.args.opts)
			if (err != nil) != tt.wantErr {
				t.Errorf("BuildPerDiscovery() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.check != nil && !tt.wantErr {
				tt.check(t, got)
			}
		})
	}
}

func Test_extractDatasetsFromModel(t *testing.T) {
	type args struct {
		modelResp *fetcher.ModelAPIResponse
		readme    *fetcher.ModelReadmeCard
	}
	tests := []struct {
		name string
		args args
		want []string
	}{
		{
			name: "no datasets",
			args: args{
				modelResp: &fetcher.ModelAPIResponse{},
				readme:    &fetcher.ModelReadmeCard{},
			},
			want: nil,
		},
		{
			name: "datasets from API response - single string",
			args: args{
				modelResp: &fetcher.ModelAPIResponse{
					CardData: map[string]interface{}{
						"datasets": "dataset1",
					},
				},
				readme: nil,
			},
			want: []string{"dataset1"},
		},
		{
			name: "datasets from API response - array",
			args: args{
				modelResp: &fetcher.ModelAPIResponse{
					CardData: map[string]interface{}{
						"datasets": []interface{}{"dataset1", "dataset2"},
					},
				},
				readme: nil,
			},
			want: []string{"dataset1", "dataset2"},
		},
		{
			name: "datasets from readme",
			args: args{
				modelResp: nil,
				readme: &fetcher.ModelReadmeCard{
					Datasets: []string{"readme-dataset1", "readme-dataset2"},
				},
			},
			want: []string{"readme-dataset1", "readme-dataset2"},
		},
		{
			name: "datasets from both sources - deduplication",
			args: args{
				modelResp: &fetcher.ModelAPIResponse{
					CardData: map[string]interface{}{
						"datasets": []interface{}{"dataset1", "dataset2"},
					},
				},
				readme: &fetcher.ModelReadmeCard{
					Datasets: []string{"dataset2", "dataset3"},
				},
			},
			want: []string{"dataset1", "dataset2", "dataset3"},
		},
		{
			name: "filters empty strings",
			args: args{
				modelResp: &fetcher.ModelAPIResponse{
					CardData: map[string]interface{}{
						"datasets": []interface{}{"dataset1", "", "  ", "dataset2"},
					},
				},
				readme: nil,
			},
			want: []string{"dataset1", "dataset2"},
		},
		{
			name: "trims whitespace",
			args: args{
				modelResp: &fetcher.ModelAPIResponse{
					CardData: map[string]interface{}{
						"datasets": []interface{}{"  dataset1  ", "dataset2"},
					},
				},
				readme: nil,
			},
			want: []string{"dataset1", "dataset2"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractDatasetsFromModel(tt.args.modelResp, tt.args.readme)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("extractDatasetsFromModel() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBuildFromModelIDs(t *testing.T) {
	// Save originals and restore after each test.
	originalBuilder := newBOMBuilder
	originalFetcherSet := newFetcherSet
	defer func() {
		newBOMBuilder = originalBuilder
		newFetcherSet = originalFetcherSet
	}()

	type args struct {
		modelIDs []string
		opts     GenerateOptions
	}
	tests := []struct {
		name    string
		args    args
		setup   func()
		wantErr bool
		check   func(*testing.T, []DiscoveredBOM)
	}{
		{
			name: "builds BOM for single model ID",
			args: args{
				modelIDs: []string{"org/model"},
				opts:     GenerateOptions{Timeout: 1 * time.Second},
			},
			setup: func() {
				newBOMBuilder = func() bomBuilder {
					return &mockBOMBuilder{
						buildFunc: func(bctx builder.BuildContext) (*cdx.BOM, error) {
							return &cdx.BOM{SerialNumber: "test"}, nil
						},
					}
				}
			},
			wantErr: false,
			check: func(t *testing.T, got []DiscoveredBOM) {
				if len(got) != 1 {
					t.Errorf("Expected 1 BOM, got %d", len(got))
					return
				}
				if got[0].Discovery.ID != "org/model" {
					t.Errorf("Expected model ID 'org/model', got %q", got[0].Discovery.ID)
				}
			},
		},
		{
			name: "skips empty model IDs",
			args: args{
				modelIDs: []string{"", "  ", "org/model"},
				opts:     GenerateOptions{Timeout: 1 * time.Second},
			},
			setup: func() {
				newBOMBuilder = func() bomBuilder {
					return &mockBOMBuilder{
						buildFunc: func(bctx builder.BuildContext) (*cdx.BOM, error) {
							return &cdx.BOM{}, nil
						},
					}
				}
			},
			wantErr: false,
			check: func(t *testing.T, got []DiscoveredBOM) {
				if len(got) != 1 {
					t.Errorf("Expected 1 BOM (empty strings skipped), got %d", len(got))
				}
			},
		},
		{
			name: "handles empty list",
			args: args{
				modelIDs: []string{},
				opts:     GenerateOptions{Timeout: 1 * time.Second},
			},
			setup: func() {
				newBOMBuilder = func() bomBuilder { return &mockBOMBuilder{} }
			},
			wantErr: false,
			check: func(t *testing.T, got []DiscoveredBOM) {
				if len(got) != 0 {
					t.Errorf("Expected 0 BOMs, got %d", len(got))
				}
			},
		},
		{
			name: "calls progress callback",
			args: args{
				modelIDs: []string{"org/model"},
				opts: GenerateOptions{
					Timeout:    1 * time.Second,
					OnProgress: func(event ProgressEvent) {},
				},
			},
			setup: func() {
				newBOMBuilder = func() bomBuilder {
					return &mockBOMBuilder{
						buildFunc: func(bctx builder.BuildContext) (*cdx.BOM, error) {
							return &cdx.BOM{}, nil
						},
					}
				}
			},
			wantErr: false,
			check: func(t *testing.T, got []DiscoveredBOM) {
				if len(got) != 1 {
					t.Errorf("Expected 1 BOM, got %d", len(got))
				}
			},
		},
		{
			name: "continues on BOM build error",
			args: args{
				modelIDs: []string{"org/model1", "org/model2"},
				opts:     GenerateOptions{Timeout: 1 * time.Second},
			},
			setup: func() {
				callCount := 0
				newBOMBuilder = func() bomBuilder {
					return &mockBOMBuilder{
						buildFunc: func(bctx builder.BuildContext) (*cdx.BOM, error) {
							callCount++
							if callCount == 1 {
								return nil, context.Canceled // Error on first
							}
							return &cdx.BOM{}, nil
						},
					}
				}
			},
			wantErr: false,
			check: func(t *testing.T, got []DiscoveredBOM) {
				if len(got) != 1 {
					t.Errorf("Expected 1 BOM (first failed, second succeeded), got %d", len(got))
				}
			},
		},
		{
			name: "successfully fetches and builds with datasets",
			args: args{
				modelIDs: []string{"org/model-with-datasets"},
				opts: GenerateOptions{
					HFToken:    "test-token",
					Timeout:    1 * time.Second,
					OnProgress: func(event ProgressEvent) {},
				},
			},
			setup: func() {
				newBOMBuilder = func() bomBuilder {
					return &mockBOMBuilder{
						buildFunc: func(bctx builder.BuildContext) (*cdx.BOM, error) {
							return &cdx.BOM{}, nil
						},
						buildDatasetFunc: func(bctx builder.DatasetBuildContext) (*cdx.Component, error) {
							return &cdx.Component{Name: bctx.DatasetID, Type: cdx.ComponentTypeData}, nil
						},
					}
				}
				newFetcherSet = func(httpClient *http.Client) fetcherSet {
					return fetcherSet{
						modelAPI: &mockModelAPIFetcher{
							fetchFunc: func(id string) (*fetcher.ModelAPIResponse, error) {
								return &fetcher.ModelAPIResponse{
									ID: id,
									CardData: map[string]interface{}{
										"datasets": []interface{}{"dataset1", "dataset2"},
									},
								}, nil
							},
						},
						modelReadme: &mockModelReadmeFetcher{
							fetchFunc: func(id string) (*fetcher.ModelReadmeCard, error) {
								return &fetcher.ModelReadmeCard{Datasets: []string{"dataset3"}}, nil
							},
						},
						datasetAPI: &mockDatasetAPIFetcher{
							fetchFunc: func(id string) (*fetcher.DatasetAPIResponse, error) {
								return &fetcher.DatasetAPIResponse{ID: id}, nil
							},
						},
						datasetReadme: &mockDatasetReadmeFetcher{
							fetchFunc: func(id string) (*fetcher.DatasetReadmeCard, error) {
								return &fetcher.DatasetReadmeCard{}, nil
							},
						},
					}
				}
			},
			wantErr: false,
			check: func(t *testing.T, got []DiscoveredBOM) {
				if len(got) != 1 {
					t.Errorf("Expected 1 BOM, got %d", len(got))
					return
				}
				// dataset1, dataset2, dataset3.
				if got[0].BOM.Components == nil || len(*got[0].BOM.Components) != 3 {
					t.Errorf("Expected 3 dataset components, got %v", got[0].BOM.Components)
				}
			},
		},
		{
			name: "handles dataset readme fetch errors",
			args: args{
				modelIDs: []string{"org/model-with-dataset-readme-error"},
				opts:     GenerateOptions{HFToken: "test-token", Timeout: 1 * time.Second},
			},
			setup: func() {
				newBOMBuilder = func() bomBuilder {
					return &mockBOMBuilder{
						buildFunc: func(bctx builder.BuildContext) (*cdx.BOM, error) {
							return &cdx.BOM{}, nil
						},
						buildDatasetFunc: func(bctx builder.DatasetBuildContext) (*cdx.Component, error) {
							return &cdx.Component{Name: bctx.DatasetID, Type: cdx.ComponentTypeData}, nil
						},
					}
				}
				newFetcherSet = func(httpClient *http.Client) fetcherSet {
					return fetcherSet{
						modelAPI: &mockModelAPIFetcher{
							fetchFunc: func(id string) (*fetcher.ModelAPIResponse, error) {
								return &fetcher.ModelAPIResponse{
									CardData: map[string]interface{}{"datasets": []interface{}{"dataset1"}},
								}, nil
							},
						},
						modelReadme: &mockModelReadmeFetcher{},
						datasetAPI: &mockDatasetAPIFetcher{
							fetchFunc: func(id string) (*fetcher.DatasetAPIResponse, error) {
								return &fetcher.DatasetAPIResponse{ID: id}, nil
							},
						},
						datasetReadme: &mockDatasetReadmeFetcher{
							fetchFunc: func(id string) (*fetcher.DatasetReadmeCard, error) {
								return nil, context.Canceled // Readme fetch fails; component still built
							},
						},
					}
				}
			},
			wantErr: false,
			check: func(t *testing.T, got []DiscoveredBOM) {
				if len(got) != 1 {
					t.Errorf("Expected 1 BOM, got %d", len(got))
					return
				}
				if got[0].BOM.Components == nil || len(*got[0].BOM.Components) != 1 {
					t.Errorf("Expected 1 dataset component (readme failure is ignored), got %v", got[0].BOM.Components)
				}
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setup != nil {
				tt.setup()
			}
			got, err := BuildFromModelIDs(tt.args.modelIDs, tt.args.opts)
			if (err != nil) != tt.wantErr {
				t.Errorf("BuildFromModelIDs() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.check != nil && !tt.wantErr {
				tt.check(t, got)
			}
		})
	}
}
