package fetcher

// DummyModelTreeFetcher returns a fixed, clean tree response for testing/demo.
// purposes without making any HTTP requests. It returns an empty slice,.
// indicating no security findings, which is the expected state for a dummy BOM.
type DummyModelTreeFetcher struct{}

// Fetch returns an empty file list, indicating a clean security scan.
func (f *DummyModelTreeFetcher) Fetch(_ string) ([]SecurityFileEntry, error) {
	return []SecurityFileEntry{}, nil
}
