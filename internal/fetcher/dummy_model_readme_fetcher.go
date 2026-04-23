package fetcher

// DummyModelReadmeFetcher returns a fixed ModelReadmeCard for testing/demo purposes.
// without making any HTTP requests.
type DummyModelReadmeFetcher struct{}

// Fetch returns a comprehensive dummy ModelReadmeCard with all fields populated.
func (f *DummyModelReadmeFetcher) Fetch(modelID string) (*ModelReadmeCard, error) {
	// Create a comprehensive dummy README card with all fields populated.
	return &ModelReadmeCard{
		Raw: dummyReadmeContent,
		FrontMatter: map[string]any{
			"language": "en",
			"license":  "mit",
			"tags":     []string{"text-generation", "pytorch", "gpt2"},
			"datasets": []string{"wikipedia", "bookcorpus"},
			"metrics":  []string{"perplexity", "accuracy"},
		},
		Body: dummyReadmeContent,

		// Common front matter fields.
		License:   "mit",
		Tags:      []string{"text-generation", "pytorch", "gpt2"},
		Datasets:  []string{"wikipedia", "bookcorpus"},
		Metrics:   []string{"perplexity", "accuracy"},
		BaseModel: "gpt2",

		// Extracted from Markdown body (template-based).
		DevelopedBy:          "Dummy Organization",
		PaperURL:             "https://arxiv.org/abs/1234.56789",
		DemoURL:              "https://huggingface.co/spaces/dummy-org/dummy-demo",
		DirectUse:            "This model is intended for text generation tasks in English. It can be used for creative writing, code generation, and general-purpose text completion.",
		OutOfScopeUse:        "This model should not be used for generating harmful content, medical advice, or making critical decisions without human oversight.",
		BiasRisksLimitations: "The model may exhibit biases present in the training data, including but not limited to gender, racial, and cultural biases. Users should be aware of potential risks when deploying in production environments.",
		BiasRecommendations:  "We recommend implementing content filtering, human review for sensitive applications, and regular bias audits when using this model in production.",
		ModelCardContact:     "contact@dummy-org.example.com",

		// Environmental Impact.
		EnvironmentalHardwareType:  "NVIDIA A100 GPU",
		EnvironmentalHoursUsed:     "168",
		EnvironmentalCloudProvider: "AWS",
		EnvironmentalComputeRegion: "us-west-2",
		EnvironmentalCarbonEmitted: "42.5 kg CO2eq",

		// From model-index.
		TaskType: "text-generation",
		TaskName: "Text Generation",
		ModelIndexMetrics: []ModelIndexMetric{
			{Type: "perplexity", Value: "15.2"},
			{Type: "accuracy", Value: "0.85"},
			{Type: "bleu", Value: "32.4"},
		},

		// Quantitative Analysis sections.
		TestingMetrics: "The model was evaluated using perplexity on a held-out test set, achieving a score of 15.2. Additional metrics include accuracy (0.85) and BLEU score (32.4) on various benchmarks.",
		Results:        "The model demonstrates strong performance on text generation tasks, with competitive results on standard benchmarks. It shows particular strength in maintaining coherence over longer sequences.",
	}, nil
}

const dummyReadmeContent = `---
language: en
license: mit
tags:
  - text-generation
  - pytorch
  - gpt2
datasets:
  - wikipedia
  - bookcorpus
metrics:
  - perplexity
  - accuracy
base_model: gpt2
model-index:
  - name: dummy-model
    results:
      - task:
          type: text-generation
          name: Text Generation
        metrics:
          - type: perplexity
            value: 15.2
          - type: accuracy
            value: 0.85
          - type: bleu
            value: 32.4
---

# Dummy Model Card

## Model Details

### Model Description

This is a comprehensive dummy model for testing and demonstration purposes. It includes all standard fields and metadata that would typically be found in a production model card.

- **Developed by:** Dummy Organization
- **Model type:** Language Model
- **Language(s):** English
- **License:** MIT
- **Paper:** https://arxiv.org/abs/1234.56789
- **Demo:** https://huggingface.co/spaces/dummy-org/dummy-demo

## Uses

### Direct Use

This model is intended for text generation tasks in English. It can be used for creative writing, code generation, and general-purpose text completion.

### Out-of-Scope Use

This model should not be used for generating harmful content, medical advice, or making critical decisions without human oversight.

## Bias, Risks, and Limitations

The model may exhibit biases present in the training data, including but not limited to gender, racial, and cultural biases. Users should be aware of potential risks when deploying in production environments.

### Recommendations

We recommend implementing content filtering, human review for sensitive applications, and regular bias audits when using this model in production.

## Training Details

### Training Data

The model was trained on a combination of Wikipedia and BookCorpus datasets, totaling approximately 10GB of text data.

### Training Procedure

- **Training regime:** Mixed precision (FP16/FP32)
- **Batch size:** 64
- **Learning rate:** 5e-5
- **Epochs:** 3

## Evaluation

### Testing Data & Metrics

#### Testing Data

The model was evaluated on a held-out test set comprising 5% of the original training data.

#### Metrics

The model was evaluated using perplexity on a held-out test set, achieving a score of 15.2. Additional metrics include accuracy (0.85) and BLEU score (32.4) on various benchmarks.

### Results

The model demonstrates strong performance on text generation tasks, with competitive results on standard benchmarks. It shows particular strength in maintaining coherence over longer sequences.

## Environmental Impact

- **Hardware Type:** NVIDIA A100 GPU
- **Hours used:** 168
- **Cloud Provider:** AWS
- **Compute Region:** us-west-2
- **Carbon Emitted:** 42.5 kg CO2eq

## Technical Specifications

### Model Architecture

The model uses a standard GPT-2 architecture with the following specifications:
- Layers: 12
- Hidden size: 768
- Attention heads: 12
- Parameters: 117M

### Compute Infrastructure

Training was performed on AWS infrastructure using NVIDIA A100 GPUs.

## Model Card Contact

contact@dummy-org.example.com
`
