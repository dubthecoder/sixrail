package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/teclara/railsix/shared/models"
)

// httpRouteLookup resolves route IDs by calling the gtfs-static service over HTTP.
// Routes are cached in-memory since they only change on GTFS refresh (every 24h).
type httpRouteLookup struct {
	baseURL string
	client  *http.Client
	mu      sync.RWMutex
	cache   map[string]models.Route
}

func newHTTPRouteLookup(baseURL string) *httpRouteLookup {
	return &httpRouteLookup{
		baseURL: baseURL,
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
		cache: make(map[string]models.Route),
	}
}

// GetRoute fetches a route by ID, returning a cached result if available.
func (l *httpRouteLookup) GetRoute(id string) (models.Route, bool) {
	l.mu.RLock()
	if route, ok := l.cache[id]; ok {
		l.mu.RUnlock()
		return route, true
	}
	l.mu.RUnlock()

	url := fmt.Sprintf("%s/routes/%s", l.baseURL, id)
	resp, err := l.client.Get(url)
	if err != nil {
		slog.Debug("route lookup request failed", "routeID", id, "error", err)
		return models.Route{}, false
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return models.Route{}, false
	}

	var route models.Route
	if err := json.NewDecoder(resp.Body).Decode(&route); err != nil {
		slog.Debug("route lookup decode failed", "routeID", id, "error", err)
		return models.Route{}, false
	}

	l.mu.Lock()
	l.cache[id] = route
	l.mu.Unlock()
	return route, true
}
