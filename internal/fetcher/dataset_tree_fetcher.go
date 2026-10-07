package fetcher

import (
	"net/http"
)

// DatasetTreeFetcher fetches the file tree with security metadata from the HF Hub.
// datasets API (/api/datasets/{id}/tree/main).
type DatasetTreeFetcher struct {
	Client  *http.Client
	BaseURL string // optional; defaults to "https://huggingface.co"
}

// Fetch returns all file entries for the given datasetID from the HF datasets tree.
// API (branch: main, expand=true, recursive=true). It follows cursor-based.
// pagination up to maxTreePages pages.
func (f *DatasetTreeFetcher) Fetch(datasetID string) ([]SecurityFileEntry, error) {
	return fetchTree(httpClient(f.Client), HFBaseURL(f.BaseURL)+"/api/datasets/"+datasetID+"/tree/main", "dataset tree")
}
