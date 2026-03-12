package store

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	neturl "net/url"
	"time"
)

var allowedGTFSHosts = map[string]bool{
	"assets.metrolinx.com":   true,
	"www.metrolinx.com":      true,
	"metrolinx.com":          true,
	"api.openmetrolinx.com":  true,
	"opendata.metrolinx.com": true,
	"gtfs.metrolinx.com":     true,
}

// DownloadURL fetches a URL and returns the response body, validating the host
// against an allowlist and enforcing a 50 MB size limit.
func DownloadURL(ctx context.Context, rawURL string) ([]byte, error) {
	parsed, err := neturl.Parse(rawURL)
	if err != nil || (parsed.Scheme != "https" && parsed.Scheme != "http") {
		return nil, fmt.Errorf("invalid or non-HTTP(S) URL: %s", rawURL)
	}
	if !allowedGTFSHosts[parsed.Hostname()] {
		return nil, fmt.Errorf("host %q not in GTFS allowlist", parsed.Hostname())
	}
	client := &http.Client{Timeout: 60 * time.Second}
	cleanURL := parsed.String()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, cleanURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request for %s: %w", rawURL, err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("downloading %s: %w", rawURL, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status %d for %s", resp.StatusCode, rawURL)
	}
	const maxBytes = 50 * 1024 * 1024 // 50 MB
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxBytes))
	if err != nil {
		return nil, fmt.Errorf("reading response from %s: %w", rawURL, err)
	}
	if int64(len(data)) > maxBytes {
		return nil, fmt.Errorf("response from %s exceeds %d byte limit", rawURL, maxBytes)
	}
	return data, nil
}

// ManageGTFS keeps retrying startup loads until the store becomes ready, then
// switches to the long-lived daily refresh loop.
func ManageGTFS(ctx context.Context, url string, s *StaticStore, refreshInterval time.Duration) {
	const startupRetryDelay = time.Minute

	for {
		if loadGTFSIntoStore(ctx, url, 5, s) {
			refreshLoop(ctx, url, s, refreshInterval)
			return
		}

		slog.Warn("GTFS static data unavailable after startup attempts, retrying soon", "retryIn", startupRetryDelay)
		if !sleepContext(ctx, startupRetryDelay) {
			return
		}
	}
}

func loadGTFSIntoStore(ctx context.Context, url string, maxAttempts int, s *StaticStore) bool {
	backoff := 2 * time.Second
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if ctx.Err() != nil {
			return false
		}

		slog.Info("downloading GTFS static data", "url", url, "attempt", attempt, "maxAttempts", maxAttempts)
		zipData, err := DownloadURL(ctx, url)
		if err != nil {
			slog.Error("failed to download GTFS static data", "error", err, "attempt", attempt)
			if attempt < maxAttempts {
				slog.Info("retrying GTFS download", "backoff", backoff)
				if !sleepContext(ctx, backoff) {
					return false
				}
				backoff *= 2
			}
			continue
		}

		if err := s.Refresh(zipData); err != nil {
			slog.Error("failed to parse GTFS static data", "error", err, "attempt", attempt)
			if attempt < maxAttempts {
				slog.Info("retrying GTFS download", "backoff", backoff)
				if !sleepContext(ctx, backoff) {
					return false
				}
				backoff *= 2
			}
			continue
		}

		slog.Info("GTFS static data loaded successfully")
		return true
	}

	return false
}

func refreshLoop(ctx context.Context, url string, s *StaticStore, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.Info("GTFS refresh loop stopped")
			return
		case <-ticker.C:
		}

		const maxRetries = 3
		backoff := 5 * time.Second
		var refreshed bool
		for attempt := 1; attempt <= maxRetries; attempt++ {
			if ctx.Err() != nil {
				return
			}

			slog.Info("refreshing GTFS static data", "attempt", attempt)
			data, err := DownloadURL(ctx, url)
			if err != nil {
				slog.Error("failed to download GTFS refresh", "error", err, "attempt", attempt)
				if attempt < maxRetries {
					if !sleepContext(ctx, backoff) {
						return
					}
					backoff *= 2
				}
				continue
			}

			if err := s.Refresh(data); err != nil {
				slog.Error("failed to parse GTFS refresh", "error", err, "attempt", attempt)
				if attempt < maxRetries {
					if !sleepContext(ctx, backoff) {
						return
					}
					backoff *= 2
				}
				continue
			}

			refreshed = true
			break
		}

		if !refreshed {
			slog.Warn("GTFS refresh failed after all retries, will try again next interval")
		}
	}
}

func sleepContext(ctx context.Context, d time.Duration) bool {
	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
