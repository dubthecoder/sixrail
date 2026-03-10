package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/teclara/railsix/shared/models"
)

// httpRouteLookup resolves route IDs by calling the gtfs-static service over HTTP.
type httpRouteLookup struct {
	baseURL string
	client  *http.Client
}

func newHTTPRouteLookup(baseURL string) *httpRouteLookup {
	return &httpRouteLookup{
		baseURL: baseURL,
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

// GetRoute fetches a route by ID from the gtfs-static service.
func (l *httpRouteLookup) GetRoute(id string) (models.Route, bool) {
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
	return route, true
}
