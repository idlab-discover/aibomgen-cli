package fetcher

// DummyModelAPIFetcher returns a fixed ModelAPIResponse for testing/demo purposes.
// without making any HTTP requests.
type DummyModelAPIFetcher struct{}

// Fetch returns a comprehensive dummy ModelAPIResponse with all fields populated.
func (f *DummyModelAPIFetcher) Fetch(modelID string) (*ModelAPIResponse, error) {
	// Create a fixed comprehensive response with all fields populated.
	gatedBool := false
	gated := BoolOrString{Bool: &gatedBool}

	return &ModelAPIResponse{
		ID:          "dummy-org/dummy-model",
		ModelID:     "dummy-org/dummy-model",
		Author:      "dummy-org",
		PipelineTag: "text-generation",
		LibraryName: "transformers",
		Tags: []string{
			"pytorch",
			"gpt2",
			"text-generation",
			"en",
			"license:mit",
			"autotrain_compatible",
			"endpoints_compatible",
		},
		License:     "mit",
		SHA:         "1234567890abcdef1234567890abcdef12345678",
		Downloads:   1234567,
		Likes:       890,
		LastMod:     "2024-01-15T10:30:00.000Z",
		CreatedAt:   "2023-06-01T08:00:00.000Z",
		Gated:       gated,
		Private:     false,
		Inference:   "enabled",
		UsedStorage: 523456789,
		CardData: map[string]any{
			"language":   "en",
			"license":    "mit",
			"tags":       []string{"text-generation", "pytorch"},
			"datasets":   []string{"wikipedia", "openwebtext"},
			"base_model": "gpt2",
		},
		Config: struct {
			ModelType     string   `json:"model_type"`
			Architectures []string `json:"architectures"`
		}{
			ModelType:     "gpt2",
			Architectures: []string{"GPT2LMHeadModel"},
		},
		Siblings: []struct {
			RFilename string `json:"rfilename"`
		}{{RFilename: "LICENSE"}, {RFilename: "config.json"}, {RFilename: "model.safetensors"}},
		BaseModels: &ModelBaseModels{
			Relation: "finetune",
			Models: []struct {
				ID string `json:"id"`
			}{{ID: "gpt2"}},
		},
	}, nil
}

// FetchRevision ignores the revision and returns the same dummy response.
func (f *DummyModelAPIFetcher) FetchRevision(modelID, _ string) (*ModelAPIResponse, error) {
	return f.Fetch(modelID)
}
