package metadata

import (
	"reflect"
	"sort"
	"testing"

	"github.com/idlab-discover/aibomgen-cli/internal/fetcher"
)

// verifiedPipelineTags are the Hub pipeline tags in pipelineTagIO. Each one is listed in
// huggingface.js packages/tasks/src/pipelines.ts, is not hideInModels, and has models on
// the Hub. Update this list deliberately when the map changes.
var verifiedPipelineTags = []string{
	"audio-classification", "audio-text-to-text", "audio-to-audio", "automatic-speech-recognition",
	"depth-estimation", "document-question-answering", "feature-extraction", "fill-mask",
	"image-classification", "image-feature-extraction", "image-segmentation", "image-text-to-image",
	"image-text-to-text", "image-text-to-video", "image-to-3d", "image-to-image", "image-to-text",
	"image-to-video", "keypoint-detection", "mask-generation", "object-detection", "question-answering",
	"sentence-similarity", "summarization", "table-question-answering", "tabular-classification",
	"tabular-regression", "text-classification", "text-generation", "text-ranking", "text-to-3d",
	"text-to-audio", "text-to-image", "text-to-speech", "text-to-video", "time-series-forecasting",
	"token-classification", "translation", "unconditional-image-generation", "video-classification",
	"video-text-to-text", "video-to-video", "visual-document-retrieval", "visual-question-answering",
	"voice-activity-detection", "zero-shot-classification", "zero-shot-image-classification",
	"zero-shot-object-detection",
}

func TestPipelineTagIOKeys(t *testing.T) {
	got := make([]string, 0, len(pipelineTagIO))
	for k := range pipelineTagIO {
		got = append(got, k)
	}
	sort.Strings(got)
	want := append([]string(nil), verifiedPipelineTags...)
	sort.Strings(want)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("pipelineTagIO keys differ from the verified Hub tags:\n got: %v\nwant: %v", got, want)
	}
}

func TestPipelineTagIOVocabulary(t *testing.T) {
	vocab := map[string]bool{
		fmtText: true, fmtImage: true, fmtAudio: true, fmtVideo: true, fmtTabular: true, fmtTimeSeries: true, fmt3D: true,
		fmtEmbedding: true, fmtLabel: true, fmtTokenLabels: true, fmtBoxes: true, fmtMask: true,
		fmtKeypoints: true, fmtSegments: true, fmtScore: true,
	}
	for tag, tio := range pipelineTagIO {
		if len(tio.outputs) == 0 {
			t.Errorf("%s: no outputs", tag)
		}
		if len(tio.inputs) == 0 && tag != "unconditional-image-generation" {
			t.Errorf("%s: no inputs", tag)
		}
		for _, f := range append(append([]string(nil), tio.inputs...), tio.outputs...) {
			if !vocab[f] {
				t.Errorf("%s: format %q is not in the vocabulary", tag, f)
			}
		}
	}
}

func TestPipelineTagFormats(t *testing.T) {
	tests := []struct {
		tag     string
		inputs  []string
		outputs []string
		ok      bool
	}{
		{"text-generation", []string{"text"}, []string{"text"}, true},
		{"fill-mask", []string{"text"}, []string{"text"}, true},
		{"  Fill-Mask ", []string{"text"}, []string{"text"}, true},
		{"sentence-similarity", []string{"text"}, []string{"embedding"}, true},
		{"feature-extraction", []string{"text"}, []string{"embedding"}, true},
		{"text-classification", []string{"text"}, []string{"label"}, true},
		{"token-classification", []string{"text"}, []string{"token labels"}, true},
		{"table-question-answering", []string{"tabular", "text"}, []string{"text"}, true},
		{"image-classification", []string{"image"}, []string{"label"}, true},
		{"object-detection", []string{"image"}, []string{"bounding boxes"}, true},
		{"image-segmentation", []string{"image"}, []string{"segmentation mask"}, true},
		{"image-text-to-text", []string{"image", "text"}, []string{"text"}, true},
		{"text-to-image", []string{"text"}, []string{"image"}, true},
		{"unconditional-image-generation", nil, []string{"image"}, true},
		{"automatic-speech-recognition", []string{"audio"}, []string{"text"}, true},
		{"text-to-speech", []string{"text"}, []string{"audio"}, true},
		{"voice-activity-detection", []string{"audio"}, []string{"segments"}, true},
		{"video-text-to-text", []string{"video", "text"}, []string{"text"}, true},
		{"image-to-3d", []string{"image"}, []string{"3d"}, true},
		{"time-series-forecasting", []string{"time-series"}, []string{"time-series"}, true},
		{"any-to-any", nil, nil, false},
		{"text2text-generation", nil, nil, false},
		{"reinforcement-learning", nil, nil, false},
		{"", nil, nil, false},
		{"made-up", nil, nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.tag, func(t *testing.T) {
			got, ok := pipelineTagFormats(tt.tag)
			if ok != tt.ok {
				t.Fatalf("ok = %v, want %v", ok, tt.ok)
			}
			if !reflect.DeepEqual(got.inputs, tt.inputs) || !reflect.DeepEqual(got.outputs, tt.outputs) {
				t.Fatalf("got %v -> %v, want %v -> %v", got.inputs, got.outputs, tt.inputs, tt.outputs)
			}
		})
	}
}

func TestModelParametersInputsOutputs(t *testing.T) {
	ioOf := func(src Source) (inputs, outputs []string, task string) {
		src.ModelID = "org/m"
		mp := applyModel(src).ModelCard.ModelParameters
		if mp == nil {
			return nil, nil, ""
		}
		if mp.Inputs != nil {
			for _, p := range *mp.Inputs {
				inputs = append(inputs, p.Format)
			}
		}
		if mp.Outputs != nil {
			for _, p := range *mp.Outputs {
				outputs = append(outputs, p.Format)
			}
		}
		return inputs, outputs, mp.Task
	}

	in, out, _ := ioOf(Source{HF: &fetcher.ModelAPIResponse{PipelineTag: "fill-mask"}})
	if !reflect.DeepEqual(in, []string{"text"}) || !reflect.DeepEqual(out, []string{"text"}) {
		t.Fatalf("fill-mask: %v -> %v", in, out)
	}

	in, out, task := ioOf(Source{HF: &fetcher.ModelAPIResponse{PipelineTag: "any-to-any"}})
	if in != nil || out != nil || task != "any-to-any" {
		t.Fatalf("unmapped tag: inputs=%v outputs=%v task=%q, want no I/O and the task kept", in, out, task)
	}

	in, out, _ = ioOf(Source{Readme: &fetcher.ModelReadmeCard{TaskType: "automatic-speech-recognition"}})
	if !reflect.DeepEqual(in, []string{"audio"}) || !reflect.DeepEqual(out, []string{"text"}) {
		t.Fatalf("README task type fallback: %v -> %v", in, out)
	}

	in, out, _ = ioOf(Source{HF: &fetcher.ModelAPIResponse{PipelineTag: "unconditional-image-generation"}})
	if in != nil || !reflect.DeepEqual(out, []string{"image"}) {
		t.Fatalf("unconditional generation: %v -> %v, want outputs only", in, out)
	}
}
