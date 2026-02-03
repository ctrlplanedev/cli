package apply

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

const defaultHTTPTimeout = 30 * time.Second

func isHTTPURL(input string) bool {
	parsed, err := url.Parse(input)
	if err != nil {
		return false
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return false
	}
	return parsed.Host != ""
}

// ParseInput reads a local file or HTTP(S) URL and returns parsed documents.
func ParseInput(ctx context.Context, input string) ([]Document, error) {
	if isHTTPURL(input) {
		data, err := fetchHTTP(ctx, input)
		if err != nil {
			return nil, err
		}
		return ParseYAML(data)
	}
	return ParseFile(input)
}

func fetchHTTP(ctx context.Context, input string) ([]byte, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, input, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request for %s: %w", input, err)
	}

	client := &http.Client{
		Timeout: defaultHTTPTimeout,
	}

	response, err := client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch %s: %w", input, err)
	}
	defer response.Body.Close()

	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf("failed to fetch %s: status %s", input, response.Status)
	}

	data, err := io.ReadAll(response.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", input, err)
	}

	return data, nil
}
