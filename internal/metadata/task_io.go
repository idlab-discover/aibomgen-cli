package metadata

import (
	"strings"

	cdx "github.com/CycloneDX/cyclonedx-go"
)

// taskIO holds the input and output formats a pipeline tag implies.
type taskIO struct {
	inputs  []string
	outputs []string
}

// Format vocabulary used in pipelineTagIO.
const (
	fmtText        = "text"
	fmtImage       = "image"
	fmtAudio       = "audio"
	fmtVideo       = "video"
	fmtTabular     = "tabular"
	fmtTimeSeries  = "time-series"
	fmt3D          = "3d"
	fmtEmbedding   = "embedding"
	fmtLabel       = "label"
	fmtTokenLabels = "token labels"
	fmtBoxes       = "bounding boxes"
	fmtMask        = "segmentation mask"
	fmtKeypoints   = "keypoints"
	fmtSegments    = "segments"
	fmtScore       = "score"
)

func taskIOOf(inputs []string, outputs ...string) taskIO {
	return taskIO{inputs: inputs, outputs: outputs}
}

func formats(fs ...string) []string { return fs }

// pipelineTagIO maps Hugging Face pipeline tags to model input and output formats.
// The keys are the tags of huggingface.js packages/tasks/src/pipelines.ts that are
// shown for models (not hideInModels). Tags without fixed I/O (any-to-any,
// reinforcement-learning, robotics, graph-ml) are left out on purpose: unknown tags
// produce no inputs/outputs rather than a guess.
var pipelineTagIO = map[string]taskIO{
	// Text.
	"text-generation":          taskIOOf(formats(fmtText), fmtText),
	"fill-mask":                taskIOOf(formats(fmtText), fmtText),
	"summarization":            taskIOOf(formats(fmtText), fmtText),
	"translation":              taskIOOf(formats(fmtText), fmtText),
	"question-answering":       taskIOOf(formats(fmtText), fmtText),
	"feature-extraction":       taskIOOf(formats(fmtText), fmtEmbedding),
	"sentence-similarity":      taskIOOf(formats(fmtText), fmtEmbedding),
	"text-classification":      taskIOOf(formats(fmtText), fmtLabel),
	"zero-shot-classification": taskIOOf(formats(fmtText), fmtLabel),
	"token-classification":     taskIOOf(formats(fmtText), fmtTokenLabels),
	"text-ranking":             taskIOOf(formats(fmtText), fmtScore),
	"table-question-answering": taskIOOf(formats(fmtTabular, fmtText), fmtText),

	// Vision.
	"image-classification":           taskIOOf(formats(fmtImage), fmtLabel),
	"zero-shot-image-classification": taskIOOf(formats(fmtImage, fmtText), fmtLabel),
	"object-detection":               taskIOOf(formats(fmtImage), fmtBoxes),
	"zero-shot-object-detection":     taskIOOf(formats(fmtImage, fmtText), fmtBoxes),
	"image-segmentation":             taskIOOf(formats(fmtImage), fmtMask),
	"mask-generation":                taskIOOf(formats(fmtImage), fmtMask),
	"keypoint-detection":             taskIOOf(formats(fmtImage), fmtKeypoints),
	"image-feature-extraction":       taskIOOf(formats(fmtImage), fmtEmbedding),
	"visual-document-retrieval":      taskIOOf(formats(fmtImage, fmtText), fmtEmbedding),
	"image-to-text":                  taskIOOf(formats(fmtImage), fmtText),
	"image-text-to-text":             taskIOOf(formats(fmtImage, fmtText), fmtText),
	"visual-question-answering":      taskIOOf(formats(fmtImage, fmtText), fmtText),
	"document-question-answering":    taskIOOf(formats(fmtImage, fmtText), fmtText),
	"image-to-image":                 taskIOOf(formats(fmtImage), fmtImage),
	"depth-estimation":               taskIOOf(formats(fmtImage), fmtImage),
	"image-text-to-image":            taskIOOf(formats(fmtImage, fmtText), fmtImage),
	"unconditional-image-generation": taskIOOf(nil, fmtImage),
	"text-to-image":                  taskIOOf(formats(fmtText), fmtImage),

	// Audio.
	"automatic-speech-recognition": taskIOOf(formats(fmtAudio), fmtText),
	"audio-text-to-text":           taskIOOf(formats(fmtAudio, fmtText), fmtText),
	"audio-classification":         taskIOOf(formats(fmtAudio), fmtLabel),
	"voice-activity-detection":     taskIOOf(formats(fmtAudio), fmtSegments),
	"text-to-speech":               taskIOOf(formats(fmtText), fmtAudio),
	"text-to-audio":                taskIOOf(formats(fmtText), fmtAudio),
	"audio-to-audio":               taskIOOf(formats(fmtAudio), fmtAudio),

	// Video.
	"video-classification": taskIOOf(formats(fmtVideo), fmtLabel),
	"video-text-to-text":   taskIOOf(formats(fmtVideo, fmtText), fmtText),
	"text-to-video":        taskIOOf(formats(fmtText), fmtVideo),
	"image-to-video":       taskIOOf(formats(fmtImage), fmtVideo),
	"image-text-to-video":  taskIOOf(formats(fmtImage, fmtText), fmtVideo),
	"video-to-video":       taskIOOf(formats(fmtVideo), fmtVideo),

	// 3D.
	"text-to-3d":  taskIOOf(formats(fmtText), fmt3D),
	"image-to-3d": taskIOOf(formats(fmtImage), fmt3D),

	// Tabular and time series.
	"tabular-classification":  taskIOOf(formats(fmtTabular), fmtLabel),
	"tabular-regression":      taskIOOf(formats(fmtTabular), fmtScore),
	"time-series-forecasting": taskIOOf(formats(fmtTimeSeries), fmtTimeSeries),
}

// pipelineTagFormats returns the input/output formats for a pipeline tag.
func pipelineTagFormats(tag string) (taskIO, bool) {
	t, ok := pipelineTagIO[strings.ToLower(strings.TrimSpace(tag))]
	return t, ok
}

// modelTaskTag returns the model's task tag, in the same order as the task FieldSpec:
// the Hub pipeline_tag, else the README model-index task type.
func modelTaskTag(src Source) string {
	if src.HF != nil {
		if s := strings.TrimSpace(src.HF.PipelineTag); s != "" {
			return s
		}
	}
	if src.Readme != nil {
		return strings.TrimSpace(src.Readme.TaskType)
	}
	return ""
}

// ioParams converts formats into CycloneDX input/output parameters.
func ioParams(fmts []string) []cdx.MLInputOutputParameters {
	out := make([]cdx.MLInputOutputParameters, 0, len(fmts))
	for _, f := range fmts {
		if f = strings.TrimSpace(f); f != "" {
			out = append(out, cdx.MLInputOutputParameters{Format: f})
		}
	}
	return out
}
