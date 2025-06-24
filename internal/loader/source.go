package loader

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// Source represents a source for loading descriptors or specifications
type Source interface {
	// Load loads content from the source
	Load(ctx context.Context, path string) (io.ReadCloser, error)
	
	// Type returns the source type name
	Type() string
}

// FileSource loads content from local file system
type FileSource struct{}

// NewFileSource creates a new file source
func NewFileSource() *FileSource {
	return &FileSource{}
}

// Load loads content from a file
func (s *FileSource) Load(ctx context.Context, path string) (io.ReadCloser, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open file %s: %w", path, err)
	}
	return file, nil
}

// Type returns the source type
func (s *FileSource) Type() string {
	return "file"
}

// HTTPSource loads content from HTTP/HTTPS endpoints
type HTTPSource struct {
	client  *http.Client
	headers map[string]string
}

// HTTPSourceConfig configures an HTTP source
type HTTPSourceConfig struct {
	Timeout time.Duration
	Headers map[string]string
}

// NewHTTPSource creates a new HTTP source
func NewHTTPSource(config *HTTPSourceConfig) *HTTPSource {
	if config == nil {
		config = &HTTPSourceConfig{
			Timeout: 30 * time.Second,
		}
	}
	
	return &HTTPSource{
		client: &http.Client{
			Timeout: config.Timeout,
		},
		headers: config.Headers,
	}
}

// Load loads content from an HTTP/HTTPS URL
func (s *HTTPSource) Load(ctx context.Context, url string) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	
	// Add custom headers
	for k, v := range s.headers {
		req.Header.Set(k, v)
	}
	
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch from %s: %w", url, err)
	}
	
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("unexpected status code %d from %s", resp.StatusCode, url)
	}
	
	return resp.Body, nil
}

// Type returns the source type
func (s *HTTPSource) Type() string {
	return "http"
}