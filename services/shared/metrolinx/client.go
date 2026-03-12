// api/internal/metrolinx/client.go
package metrolinx

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

func NewClient(baseURL, apiKey string) *Client {
	return &Client{
		baseURL: baseURL,
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (c *Client) Fetch(ctx context.Context, path string) ([]byte, error) {
	url := fmt.Sprintf("%s%s?key=%s", c.baseURL, path, c.apiKey)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d for %s", resp.StatusCode, path)
	}

	const maxBodyBytes = 10 * 1024 * 1024 // 10 MB
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxBodyBytes))
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}
	if int64(len(data)) > maxBodyBytes {
		return nil, fmt.Errorf("response for %s exceeds %d byte limit", path, maxBodyBytes)
	}
	return data, nil
}
