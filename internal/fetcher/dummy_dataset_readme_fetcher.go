package fetcher

// DummyDatasetReadmeFetcher returns a fixed DatasetReadmeCard for testing/demo purposes.
type DummyDatasetReadmeFetcher struct{}

// Fetch returns a dummy dataset README card.
func (f *DummyDatasetReadmeFetcher) Fetch(datasetID string) (*DatasetReadmeCard, error) {
	return &DatasetReadmeCard{
		Raw: "# " + datasetID + " Dataset\n\nDummy dataset card for testing.",
		FrontMatter: map[string]any{
			"license": "cc0-1.0",
			"tags":    []string{"dataset", "test"},
		},
		License:            "cc0-1.0",
		Tags:               []string{"dataset", "test"},
		Language:           []string{"en"},
		AnnotationCreators: []string{"Test Annotator", "Secondary Annotator"},
		Configs: []DatasetConfig{
			{
				Name: "default",
				DataFiles: []DatasetDataFile{
					{Split: "train", Path: "data/train.csv"},
					{Split: "test", Path: "data/test.csv"},
				},
			},
		},
		DatasetDescription:    "A dummy dataset for testing dataset component building with comprehensive metadata",
		CuratedBy:             "Dummy Curator",
		FundedBy:              "Test Foundation",
		SharedBy:              "Test Team",
		RepositoryURL:         "https://huggingface.co/datasets/" + datasetID,
		PaperURL:              "https://arxiv.org/abs/2401.12345",
		DemoURL:               "https://huggingface.co/spaces/demo/" + datasetID,
		OutOfScopeUse:         "This dataset should not be used for production systems without proper validation",
		PersonalSensitiveInfo: "This dataset may contain synthetic personal information for testing purposes",
		BiasRisksLimitations:  "Dataset may contain inherent biases from the synthetic generation process",
		DatasetCardContact:    "test@example.com",
	}, nil
}
