package builder

import (
	"github.com/idlab-discover/aibomgen-cli/internal/fetcher"
	"github.com/idlab-discover/aibomgen-cli/pkg/aibomgen/scanner"
)

type BuildContext struct {
	ModelID string
	// Revision is the requested model revision (branch, tag or commit); empty means the default branch.
	Revision     string
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
	HuggingFaceBaseURL string
}

func DefaultOptions() Options {
	return Options{
		HuggingFaceBaseURL: "https://huggingface.co/",
	}
}
