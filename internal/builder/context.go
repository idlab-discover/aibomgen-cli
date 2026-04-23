package builder

import (
	"github.com/idlab-discover/aibomgen-cli/internal/fetcher"
	"github.com/idlab-discover/aibomgen-cli/pkg/aibomgen/scanner"
)

type BuildContext struct {
	ModelID      string
	Scan         scanner.Discovery
	HF           *fetcher.ModelAPIResponse
	Readme       *fetcher.ModelReadmeCard
	SecurityTree []fetcher.SecurityFileEntry
}

// DatasetBuildContext for dataset component building.
type DatasetBuildContext struct {
	DatasetID string
	Scan      scanner.Discovery
	HF        *fetcher.DatasetAPIResponse
	Readme    *fetcher.DatasetReadmeCard
}

type Options struct {
	IncludeEvidenceProperties bool
	HuggingFaceBaseURL        string
}

func DefaultOptions() Options {
	return Options{
		IncludeEvidenceProperties: true,
		HuggingFaceBaseURL:        "https://huggingface.co/",
	}
}
