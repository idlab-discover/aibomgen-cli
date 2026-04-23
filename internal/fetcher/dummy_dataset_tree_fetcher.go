package fetcher

// DummyDatasetTreeFetcher returns an empty tree response for testing/demo.
// purposes without making any HTTP requests.
type DummyDatasetTreeFetcher struct{}

// Fetch returns an empty file list, indicating a clean security scan.
func (f *DummyDatasetTreeFetcher) Fetch(_ string) ([]SecurityFileEntry, error) {
	return []SecurityFileEntry{}, nil
}
