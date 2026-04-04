package api

// UploadResult holds the URL returned by catbox/litterbox along with the original filename.
type UploadResult struct {
	URL      string
	Filename string
}
