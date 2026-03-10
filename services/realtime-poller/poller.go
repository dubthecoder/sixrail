package main

import (
	"context"
	"log/slog"
	"sync"

	"github.com/teclara/railsix/shared/gtfsrt"
	"github.com/teclara/railsix/shared/metrolinx"
	"github.com/teclara/railsix/shared/models"
)

// Fetcher abstracts the HTTP fetch used by the metrolinx client.
type Fetcher interface {
	Fetch(ctx context.Context, path string) ([]byte, error)
}

// RouteLookup resolves route IDs to Route structs.
type RouteLookup interface {
	GetRoute(id string) (models.Route, bool)
}

// PollResult holds results from parallel fetches.
type PollResult struct {
	mu                 sync.Mutex
	alerts             []models.Alert
	hasAlerts          bool
	tripUpdates        map[string]gtfsrt.RawTripUpdate
	hasTripUpdates     bool
	serviceGlance      []models.ServiceGlanceEntry
	hasServiceGlance   bool
	exceptions         map[string]bool
	hasExceptions      bool
	unionDepartures    []models.UnionDeparture
	hasUnionDepartures bool
}

// fetchAlerts fetches and parses the GTFS-RT alerts feed.
func fetchAlerts(ctx context.Context, fetcher Fetcher) ([]gtfsrt.RawAlert, error) {
	data, err := fetcher.Fetch(ctx, "/Gtfs/Feed/Alerts")
	if err != nil {
		return nil, err
	}
	return gtfsrt.ParseAlerts(data)
}

// fetchTripUpdates fetches and parses the GTFS-RT trip updates feed.
func fetchTripUpdates(ctx context.Context, fetcher Fetcher) (map[string]gtfsrt.RawTripUpdate, error) {
	data, err := fetcher.Fetch(ctx, "/Gtfs/Feed/TripUpdates")
	if err != nil {
		return nil, err
	}
	return gtfsrt.ParseTripUpdates(data)
}

// pollAll launches parallel goroutines to fetch all realtime data sources.
// includeExceptions controls whether the exceptions endpoint is polled (every other tick).
func pollAll(ctx context.Context, mx *metrolinx.Client, lookup RouteLookup, includeExceptions bool) *PollResult {
	result := &PollResult{}
	var wg sync.WaitGroup

	// 1. Alerts
	wg.Add(1)
	go func() {
		defer wg.Done()
		raw, err := fetchAlerts(ctx, mx)
		if err != nil {
			slog.Error("poll alerts failed", "error", err)
			return
		}
		enriched := gtfsrt.EnrichAlerts(raw, lookup)
		result.mu.Lock()
		result.alerts = enriched
		result.hasAlerts = true
		result.mu.Unlock()
	}()

	// 2. Trip updates
	wg.Add(1)
	go func() {
		defer wg.Done()
		updates, err := fetchTripUpdates(ctx, mx)
		if err != nil {
			slog.Error("poll trip updates failed", "error", err)
			return
		}
		result.mu.Lock()
		result.tripUpdates = updates
		result.hasTripUpdates = true
		result.mu.Unlock()
	}()

	// 3. Service glance
	wg.Add(1)
	go func() {
		defer wg.Done()
		entries, err := mx.GetServiceGlance(ctx)
		if err != nil {
			slog.Error("poll service glance failed", "error", err)
			return
		}
		result.mu.Lock()
		result.serviceGlance = entries
		result.hasServiceGlance = true
		result.mu.Unlock()
	}()

	// 4. Union departures
	wg.Add(1)
	go func() {
		defer wg.Done()
		deps, err := mx.GetUnionDepartures(ctx)
		if err != nil {
			slog.Error("poll union departures failed", "error", err)
			return
		}
		result.mu.Lock()
		result.unionDepartures = deps
		result.hasUnionDepartures = true
		result.mu.Unlock()
	}()

	// 5. Exceptions (only every other tick)
	if includeExceptions {
		wg.Add(1)
		go func() {
			defer wg.Done()
			cancelled, err := mx.GetExceptions(ctx)
			if err != nil {
				slog.Error("poll exceptions failed", "error", err)
				return
			}
			result.mu.Lock()
			result.exceptions = cancelled
			result.hasExceptions = true
			result.mu.Unlock()
		}()
	}

	wg.Wait()
	return result
}

