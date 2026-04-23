package metadata

import (
	"fmt"
	"strings"

	cdx "github.com/CycloneDX/cyclonedx-go"
)

func modelCardFields() []FieldSpec {
	return []FieldSpec{
		{
			Key:      ModelCardModelParametersTask,
			Weight:   1.0,
			Required: false,
			Sources: []func(Source) (any, bool){
				func(src Source) (any, bool) {
					if src.HF == nil {
						return nil, false
					}
					s := strings.TrimSpace(src.HF.PipelineTag)
					if s == "" {
						return nil, false
					}
					return s, true
				},
				func(src Source) (any, bool) {
					if src.Readme == nil {
						return nil, false
					}
					s := strings.TrimSpace(src.Readme.TaskType)
					if s == "" {
						return nil, false
					}
					return s, true
				},
			},
			Parse: func(value string) (any, error) {
				return parseNonEmptyString(value, "task")
			},
			Apply: func(tgt Target, value any) error {
				input, ok := value.(applyInput)
				if !ok {
					return fmt.Errorf("invalid input for %s", ModelCardModelParametersTask)
				}
				if tgt.ModelCard == nil {
					return fmt.Errorf("modelCard is nil")
				}
				if !input.Force {
					if mp := bomModelParameters(tgt.BOM); mp != nil && strings.TrimSpace(mp.Task) != "" {
						return nil
					}
				}
				s, _ := input.Value.(string)
				s = strings.TrimSpace(s)
				if s == "" {
					return fmt.Errorf("task value is empty")
				}
				mp := ensureModelParameters(tgt.ModelCard)
				mp.Task = s
				return nil
			},
			Present: func(b *cdx.BOM) bool {
				mp := bomModelParameters(b)
				ok := mp != nil && strings.TrimSpace(mp.Task) != ""
				return ok
			},
			InputType:   InputTypeSelect,
			Placeholder: "Select the primary task",
			Suggestions: []string{"text-classification", "text-generation", "token-classification", "question-answering", "summarization", "translation", "image-classification", "object-detection", "image-segmentation", "audio-classification", "automatic-speech-recognition"},
		},
		{
			Key:      ModelCardModelParametersArchitectureFamily,
			Weight:   0.5,
			Required: false,
			Sources: []func(Source) (any, bool){
				func(src Source) (any, bool) {
					if src.HF == nil {
						return nil, false
					}
					s := strings.TrimSpace(src.HF.Config.ModelType)
					if s == "" {
						return nil, false
					}
					return s, true
				},
			},
			Parse: func(value string) (any, error) {
				return parseNonEmptyString(value, "architectureFamily")
			},
			Apply: func(tgt Target, value any) error {
				input, ok := value.(applyInput)
				if !ok {
					return fmt.Errorf("invalid input for %s", ModelCardModelParametersArchitectureFamily)
				}
				if tgt.ModelCard == nil {
					return fmt.Errorf("modelCard is nil")
				}
				s, _ := input.Value.(string)
				s = strings.TrimSpace(s)
				if s == "" {
					return fmt.Errorf("architectureFamily value is empty")
				}
				mp := ensureModelParameters(tgt.ModelCard)
				mp.ArchitectureFamily = s
				return nil
			},
			Present: func(b *cdx.BOM) bool {
				mp := bomModelParameters(b)
				ok := mp != nil && strings.TrimSpace(mp.ArchitectureFamily) != ""
				return ok
			},
			InputType:   InputTypeText,
			Placeholder: "e.g., transformer, cnn, rnn",
			Suggestions: []string{"transformer", "cnn", "rnn", "lstm", "gru", "diffusion"},
		},
		{
			Key:      ModelCardModelParametersModelArchitecture,
			Weight:   0.5,
			Required: false,
			Sources: []func(Source) (any, bool){
				func(src Source) (any, bool) {
					if src.HF == nil {
						return nil, false
					}
					if len(src.HF.Config.Architectures) == 0 {
						return nil, false
					}
					s := strings.TrimSpace(src.HF.Config.Architectures[0])
					if s == "" {
						return nil, false
					}
					return s, true
				},
			},
			Parse: func(value string) (any, error) {
				return parseNonEmptyString(value, "modelArchitecture")
			},
			Apply: func(tgt Target, value any) error {
				input, ok := value.(applyInput)
				if !ok {
					return fmt.Errorf("invalid input for %s", ModelCardModelParametersModelArchitecture)
				}
				if tgt.ModelCard == nil {
					return fmt.Errorf("modelCard is nil")
				}
				s, _ := input.Value.(string)
				s = strings.TrimSpace(s)
				if s == "" {
					return fmt.Errorf("modelArchitecture value is empty")
				}
				mp := ensureModelParameters(tgt.ModelCard)
				mp.ModelArchitecture = s
				return nil
			},
			Present: func(b *cdx.BOM) bool {
				mp := bomModelParameters(b)
				ok := mp != nil && strings.TrimSpace(mp.ModelArchitecture) != ""
				return ok
			},
			InputType:   InputTypeText,
			Placeholder: "e.g., BertForSequenceClassification",
		},
		{
			Key:      ModelCardModelParametersDatasets,
			Weight:   0.5,
			Required: false,
			Sources: []func(Source) (any, bool){
				func(src Source) (any, bool) {
					if src.HF == nil {
						return nil, false
					}
					ds := extractDatasets(src.HF.CardData, src.HF.Tags)
					if len(ds) == 0 {
						return nil, false
					}
					choices := make([]cdx.MLDatasetChoice, 0, len(ds))
					for _, ref := range ds {
						ref = strings.TrimSpace(ref)
						choices = append(choices, cdx.MLDatasetChoice{Ref: ref})
					}
					return choices, true
				},
				func(src Source) (any, bool) {
					if src.Readme == nil {
						return nil, false
					}
					ds := normalizeStrings(src.Readme.Datasets)
					if len(ds) == 0 {
						return nil, false
					}
					for i := range ds {
						ds[i] = normalizeDatasetRef(ds[i])
					}
					choices := make([]cdx.MLDatasetChoice, 0, len(ds))
					for _, ref := range ds {
						ref = strings.TrimSpace(ref)
						choices = append(choices, cdx.MLDatasetChoice{Ref: ref})
					}
					return choices, true
				},
			},
			Parse: func(value string) (any, error) {
				return parseDatasetRefs(value)
			},
			Apply: func(tgt Target, value any) error {
				input, ok := value.(applyInput)
				if !ok {
					return fmt.Errorf("invalid input for %s", ModelCardModelParametersDatasets)
				}
				if tgt.ModelCard == nil {
					return fmt.Errorf("modelCard is nil")
				}
				choices, _ := input.Value.([]cdx.MLDatasetChoice)
				if len(choices) == 0 {
					return fmt.Errorf("datasets value is empty")
				}
				if !input.Force && tgt.ModelCard.ModelParameters != nil && tgt.ModelCard.ModelParameters.Datasets != nil && len(*tgt.ModelCard.ModelParameters.Datasets) > 0 {
					return nil
				}
				mp := ensureModelParameters(tgt.ModelCard)
				mp.Datasets = &choices
				return nil
			},
			Present: func(b *cdx.BOM) bool {
				mp := bomModelParameters(b)
				if mp == nil || mp.Datasets == nil || len(*mp.Datasets) == 0 {
					return false
				}
				for _, d := range *mp.Datasets {
					if strings.TrimSpace(d.Ref) != "" {
						return true
					}
				}
				return false
			},
			InputType:   InputTypeMultiText,
			Placeholder: "dataset1, dataset2, dataset3",
		},
		{
			Key:      ModelCardConsiderationsUseCases,
			Weight:   0.5,
			Required: false,
			Sources: []func(Source) (any, bool){
				func(src Source) (any, bool) {
					if src.Readme == nil {
						return nil, false
					}
					useCases := []string{}
					if s := strings.TrimSpace(src.Readme.DirectUse); s != "" {
						useCases = append(useCases, s)
					}
					if s := strings.TrimSpace(src.Readme.OutOfScopeUse); s != "" {
						useCases = append(useCases, "out-of-scope: "+s)
					}
					useCases = normalizeStrings(useCases)
					if len(useCases) == 0 {
						return nil, false
					}
					return useCases, true
				},
			},
			Parse: func(value string) (any, error) {
				return parseCommaList(value, "useCases")
			},
			Apply: func(tgt Target, value any) error {
				input, ok := value.(applyInput)
				if !ok {
					return fmt.Errorf("invalid input for %s", ModelCardConsiderationsUseCases)
				}
				if tgt.ModelCard == nil {
					return fmt.Errorf("modelCard is nil")
				}
				useCases, _ := input.Value.([]string)
				if len(useCases) == 0 {
					return fmt.Errorf("useCases value is empty")
				}
				if !input.Force && tgt.ModelCard.Considerations != nil && tgt.ModelCard.Considerations.UseCases != nil && len(*tgt.ModelCard.Considerations.UseCases) > 0 {
					return nil
				}
				cons := ensureConsiderations(tgt.ModelCard)
				cons.UseCases = &useCases
				return nil
			},
			Present: func(b *cdx.BOM) bool {
				c := bomComponent(b)
				ok := c != nil && c.ModelCard != nil && c.ModelCard.Considerations != nil && c.ModelCard.Considerations.UseCases != nil && len(*c.ModelCard.Considerations.UseCases) > 0
				return ok
			},
			InputType:   InputTypeMultiText,
			Placeholder: "use case 1, use case 2",
		},
		{
			Key:      ModelCardConsiderationsTechnicalLimitations,
			Weight:   0.5,
			Required: false,
			Sources: []func(Source) (any, bool){
				func(src Source) (any, bool) {
					if src.Readme == nil {
						return nil, false
					}
					s := strings.TrimSpace(src.Readme.BiasRisksLimitations)
					if s == "" {
						return nil, false
					}
					return []string{s}, true
				},
			},
			Parse: func(value string) (any, error) {
				return parseCommaList(value, "technicalLimitations")
			},
			Apply: func(tgt Target, value any) error {
				input, ok := value.(applyInput)
				if !ok {
					return fmt.Errorf("invalid input for %s", ModelCardConsiderationsTechnicalLimitations)
				}
				if tgt.ModelCard == nil {
					return fmt.Errorf("modelCard is nil")
				}
				vals, _ := input.Value.([]string)
				if len(vals) == 0 {
					return fmt.Errorf("technicalLimitations value is empty")
				}
				if !input.Force && tgt.ModelCard.Considerations != nil && tgt.ModelCard.Considerations.TechnicalLimitations != nil && len(*tgt.ModelCard.Considerations.TechnicalLimitations) > 0 {
					return nil
				}
				cons := ensureConsiderations(tgt.ModelCard)
				cons.TechnicalLimitations = &vals
				return nil
			},
			Present: func(b *cdx.BOM) bool {
				c := bomComponent(b)
				ok := c != nil && c.ModelCard != nil && c.ModelCard.Considerations != nil && c.ModelCard.Considerations.TechnicalLimitations != nil && len(*c.ModelCard.Considerations.TechnicalLimitations) > 0
				return ok
			},
			InputType:   InputTypeTextArea,
			Placeholder: "limitation1,limitation2,limitation3",
		},
		{
			Key:      ModelCardConsiderationsEthicalConsiderations,
			Weight:   0.25,
			Required: false,
			Sources: []func(Source) (any, bool){
				func(src Source) (any, bool) {
					if src.Readme == nil {
						return nil, false
					}
					name := strings.TrimSpace(src.Readme.BiasRisksLimitations)
					mit := strings.TrimSpace(src.Readme.BiasRecommendations)
					if name == "" && mit == "" {
						return nil, false
					}
					if name == "" {
						name = "bias_risks_limitations"
					}
					ethics := []cdx.MLModelCardEthicalConsideration{{Name: name, MitigationStrategy: mit}}
					return ethics, true
				},
			},
			Parse: func(value string) (any, error) {
				return parseEthicalConsiderations(value)
			},
			Apply: func(tgt Target, value any) error {
				input, ok := value.(applyInput)
				if !ok {
					return fmt.Errorf("invalid input for %s", ModelCardConsiderationsEthicalConsiderations)
				}
				if tgt.ModelCard == nil {
					return fmt.Errorf("modelCard is nil")
				}
				ethics, _ := input.Value.([]cdx.MLModelCardEthicalConsideration)
				if len(ethics) == 0 {
					return fmt.Errorf("ethicalConsiderations value is empty")
				}
				if !input.Force && tgt.ModelCard.Considerations != nil && tgt.ModelCard.Considerations.EthicalConsiderations != nil && len(*tgt.ModelCard.Considerations.EthicalConsiderations) > 0 {
					return nil
				}
				cons := ensureConsiderations(tgt.ModelCard)
				cons.EthicalConsiderations = &ethics
				return nil
			},
			Present: func(b *cdx.BOM) bool {
				c := bomComponent(b)
				ok := c != nil && c.ModelCard != nil && c.ModelCard.Considerations != nil && c.ModelCard.Considerations.EthicalConsiderations != nil && len(*c.ModelCard.Considerations.EthicalConsiderations) > 0
				return ok
			},
			InputType:   InputTypeTextArea,
			Placeholder: "bias:mitigation strategy,privacy concerns,fairness issues",
		},
		{
			Key:      ModelCardQuantitativeAnalysisPerformanceMetrics,
			Weight:   0.5,
			Required: false,
			Sources: []func(Source) (any, bool){
				func(src Source) (any, bool) {
					if src.Readme == nil {
						return nil, false
					}
					metrics := make([]cdx.MLPerformanceMetric, 0)
					for _, m := range src.Readme.ModelIndexMetrics {
						mt := strings.TrimSpace(m.Type)
						mv := strings.TrimSpace(m.Value)
						if mt == "" && mv == "" {
							continue
						}
						metrics = append(metrics, cdx.MLPerformanceMetric{Type: mt, Value: mv})
					}
					for _, mt := range src.Readme.Metrics {
						mt = strings.TrimSpace(mt)
						if mt == "" {
							continue
						}
						alreadyExists := false
						for _, existing := range metrics {
							if existing.Type == mt {
								alreadyExists = true
								break
							}
						}
						if !alreadyExists {
							metrics = append(metrics, cdx.MLPerformanceMetric{Type: mt, Value: ""})
						}
					}
					if len(metrics) == 0 {
						testingMetrics := strings.TrimSpace(src.Readme.TestingMetrics)
						results := strings.TrimSpace(src.Readme.Results)
						if testingMetrics != "" || results != "" {
							metricType := "testing_metrics"
							metricValue := ""
							if testingMetrics != "" {
								metricType = testingMetrics
							}
							if results != "" {
								metricValue = results
							}
							metrics = append(metrics, cdx.MLPerformanceMetric{Type: metricType, Value: metricValue})
						}
					}
					if len(metrics) == 0 {
						return nil, false
					}
					return metrics, true
				},
			},
			Parse: func(value string) (any, error) {
				return parsePerformanceMetrics(value)
			},
			Apply: func(tgt Target, value any) error {
				input, ok := value.(applyInput)
				if !ok {
					return fmt.Errorf("invalid input for %s", ModelCardQuantitativeAnalysisPerformanceMetrics)
				}
				if tgt.ModelCard == nil {
					return fmt.Errorf("modelCard is nil")
				}
				metrics, _ := input.Value.([]cdx.MLPerformanceMetric)
				if len(metrics) == 0 {
					return fmt.Errorf("performanceMetrics value is empty")
				}
				if !input.Force && tgt.ModelCard.QuantitativeAnalysis != nil && tgt.ModelCard.QuantitativeAnalysis.PerformanceMetrics != nil && len(*tgt.ModelCard.QuantitativeAnalysis.PerformanceMetrics) > 0 {
					return nil
				}
				qa := ensureQuantitativeAnalysis(tgt.ModelCard)
				qa.PerformanceMetrics = &metrics
				return nil
			},
			Present: func(b *cdx.BOM) bool {
				c := bomComponent(b)
				ok := c != nil && c.ModelCard != nil && c.ModelCard.QuantitativeAnalysis != nil && c.ModelCard.QuantitativeAnalysis.PerformanceMetrics != nil && len(*c.ModelCard.QuantitativeAnalysis.PerformanceMetrics) > 0
				return ok
			},
			InputType:   InputTypeTextArea,
			Placeholder: "accuracy:0.95,f1:0.92,precision:0.88",
		},
		{
			Key:      ModelCardConsiderationsEnvironmentalConsiderationsProperties,
			Weight:   0.25,
			Required: false,
			Sources: []func(Source) (any, bool){
				func(src Source) (any, bool) {
					if src.Readme == nil {
						return nil, false
					}
					props := []cdx.Property{}
					add := func(name, value string) {
						name = strings.TrimSpace(name)
						value = strings.TrimSpace(value)
						if name == "" || value == "" {
							return
						}
						props = append(props, cdx.Property{Name: name, Value: value})
					}
					add("hardwareType", src.Readme.EnvironmentalHardwareType)
					add("hoursUsed", src.Readme.EnvironmentalHoursUsed)
					add("cloudProvider", src.Readme.EnvironmentalCloudProvider)
					add("computeRegion", src.Readme.EnvironmentalComputeRegion)
					add("carbonEmitted", src.Readme.EnvironmentalCarbonEmitted)
					if len(props) == 0 {
						return nil, false
					}
					return props, true
				},
			},
			Parse: func(value string) (any, error) {
				return parseProperties(value)
			},
			Apply: func(tgt Target, value any) error {
				input, ok := value.(applyInput)
				if !ok {
					return fmt.Errorf("invalid input for %s", ModelCardConsiderationsEnvironmentalConsiderationsProperties)
				}
				if tgt.ModelCard == nil {
					return fmt.Errorf("modelCard is nil")
				}
				props, _ := input.Value.([]cdx.Property)
				if len(props) == 0 {
					return fmt.Errorf("environmentalConsiderations value is empty")
				}
				if !input.Force && tgt.ModelCard.Considerations != nil && tgt.ModelCard.Considerations.EnvironmentalConsiderations != nil {
					env := tgt.ModelCard.Considerations.EnvironmentalConsiderations
					if env.Properties != nil && len(*env.Properties) > 0 {
						return nil
					}
				}
				cons := ensureConsiderations(tgt.ModelCard)
				if cons.EnvironmentalConsiderations == nil {
					cons.EnvironmentalConsiderations = &cdx.MLModelCardEnvironmentalConsiderations{}
				}
				cons.EnvironmentalConsiderations.Properties = &props
				return nil
			},
			Present: func(b *cdx.BOM) bool {
				c := bomComponent(b)
				ok := c != nil && c.ModelCard != nil && c.ModelCard.Considerations != nil && c.ModelCard.Considerations.EnvironmentalConsiderations != nil && c.ModelCard.Considerations.EnvironmentalConsiderations.Properties != nil && len(*c.ModelCard.Considerations.EnvironmentalConsiderations.Properties) > 0
				return ok
			},
			InputType:   InputTypeTextArea,
			Placeholder: "hardwareType:GPU,hoursUsed:100,carbonEmitted:50kg",
		},
	}
}
