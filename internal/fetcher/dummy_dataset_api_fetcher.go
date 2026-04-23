package fetcher

// DummyDatasetAPIFetcher returns a fixed DatasetAPIResponse for testing/demo purposes.
type DummyDatasetAPIFetcher struct{}

// Fetch returns a dummy dataset response.
func (f *DummyDatasetAPIFetcher) Fetch(datasetID string) (*DatasetAPIResponse, error) {
	return &DatasetAPIResponse{
		ID:          datasetID,
		Author:      "huggingface",
		SHA:         "abc123def456789012345678901234567890abcd",
		LastMod:     "2024-01-15T10:30:00.000Z",
		CreatedAt:   "2023-06-01T08:00:00.000Z",
		UsedStorage: 1024000,
		Tags:        []string{"dataset", "benchmark", "text-classification"},
		Description: "Dummy dataset for testing: " + datasetID,
		Downloads:   100000,
		Likes:       500,
		CardData: map[string]any{
			"language":        "en",
			"license":         "cc0-1.0",
			"task_categories": []interface{}{"text-classification", "text-generation"},
			"tags":            []interface{}{"sentiment-analysis", "benchmark"},
		},
	}, nil
}
