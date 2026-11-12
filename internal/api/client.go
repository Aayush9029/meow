package api

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ProgressFunc is called during file upload with bytes read and total size.
type ProgressFunc func(bytesRead, total int64)

// progressReader wraps an io.Reader and fires a callback on each Read.
type progressReader struct {
	reader io.Reader
	total  int64
	read   int64
	onRead ProgressFunc
}

func (pr *progressReader) Read(p []byte) (int, error) {
	n, err := pr.reader.Read(p)
	pr.read += int64(n)
	if pr.onRead != nil {
		pr.onRead(pr.read, pr.total)
	}
	return n, err
}

type Client struct {
	http *http.Client
}

func NewClient() *Client {
	// A whole-request timeout would cut off large uploads, so only the wait
	// for the response is limited.
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.ResponseHeaderTimeout = 2 * time.Minute
	return &Client{http: &http.Client{Transport: transport}}
}

// PostUpload is PostMultipart for requests that must return a URL. Catbox can
// answer 200 with an empty or plain-text body when an upload fails.
func (c *Client) PostUpload(
	apiURL string,
	fields map[string]string,
	fileField string,
	filePath string,
	onProgress ProgressFunc,
) (string, error) {
	result, err := c.PostMultipart(apiURL, fields, fileField, filePath, onProgress)
	if err != nil {
		return "", err
	}
	if result == "" {
		return "", errors.New("server returned an empty response")
	}
	if !strings.HasPrefix(result, "http://") && !strings.HasPrefix(result, "https://") {
		return "", fmt.Errorf("server did not return a URL: %s", result)
	}
	return result, nil
}

// PostMultipart sends a multipart POST request. If filePath is non-empty, the file
// is streamed via io.MultiReader (preamble buffer + progress-wrapped file + closing boundary)
// so the full body is never buffered in memory.
func (c *Client) PostMultipart(
	apiURL string,
	fields map[string]string,
	fileField string,
	filePath string,
	onProgress ProgressFunc,
) (string, error) {
	if fileField == "" || filePath == "" {
		return c.postFieldsOnly(apiURL, fields)
	}

	f, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("open %s: %w", filePath, err)
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return "", err
	}
	fileSize := info.Size()

	// Build the preamble (fields + file part header) into a buffer.
	var preamble bytes.Buffer
	mw := multipart.NewWriter(&preamble)
	boundary := mw.Boundary()

	for k, v := range fields {
		mw.WriteField(k, v)
	}
	// Write the file part header (but not the file content).
	partHeader := fmt.Sprintf("\r\n--%s\r\nContent-Disposition: form-data; name=%q; filename=%q\r\nContent-Type: application/octet-stream\r\n\r\n",
		boundary, fileField, filepath.Base(filePath))
	preamble.WriteString(partHeader)

	// Closing boundary.
	closing := []byte(fmt.Sprintf("\r\n--%s--\r\n", boundary))

	// Wrap the file reader for progress tracking.
	pr := &progressReader{reader: f, total: fileSize, onRead: onProgress}

	// Concatenate: preamble + file (with progress) + closing boundary.
	body := io.MultiReader(&preamble, pr, bytes.NewReader(closing))
	totalSize := int64(preamble.Len()) + fileSize + int64(len(closing))

	req, err := http.NewRequest("POST", apiURL, body)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "multipart/form-data; boundary="+boundary)
	req.ContentLength = totalSize

	return c.doRequest(req)
}

// postFieldsOnly handles requests with no file (URL uploads, deletes, album ops).
func (c *Client) postFieldsOnly(apiURL string, fields map[string]string) (string, error) {
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	for k, v := range fields {
		mw.WriteField(k, v)
	}
	mw.Close()

	req, err := http.NewRequest("POST", apiURL, &body)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", mw.FormDataContentType())

	return c.doRequest(req)
}

func (c *Client) doRequest(req *http.Request) (string, error) {
	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	result := strings.TrimSpace(string(respBody))
	if resp.StatusCode != 200 {
		if result == "" || strings.HasPrefix(result, "<") {
			return "", fmt.Errorf("server returned %s", resp.Status)
		}
		return "", fmt.Errorf("server returned %d: %s", resp.StatusCode, result)
	}
	return result, nil
}
