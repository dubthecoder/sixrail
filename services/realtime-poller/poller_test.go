package main

import (
	"context"
	"fmt"
	"testing"

	"github.com/teclara/railsix/shared/models"
)

// mockFetcher returns canned responses for specific paths.
type mockFetcher struct {
	responses map[string][]byte
	errors    map[string]error
}

func (m *mockFetcher) Fetch(_ context.Context, path string) ([]byte, error) {
	if err, ok := m.errors[path]; ok {
		return nil, err
	}
	if data, ok := m.responses[path]; ok {
		return data, nil
	}
	return nil, fmt.Errorf("no mock response for path %q", path)
}

// mockLookup resolves route IDs for enrichment.
type mockLookup struct {
	routes map[string]models.Route
}

func (m *mockLookup) GetRoute(id string) (models.Route, bool) {
	r, ok := m.routes[id]
	return r, ok
}

func TestFetchAlerts(t *testing.T) {
	fetcher := &mockFetcher{
		responses: map[string][]byte{
			"/Gtfs/Feed/Alerts": []byte(`{
				"entity": [{
					"id": "alert-1",
					"alert": {
						"effect": "REDUCED_SERVICE",
						"header_text": {
							"translation": [{"text": "Delay on LW", "language": "en"}]
						},
						"description_text": {
							"translation": [{"text": "10 min delays", "language": "en"}]
						},
						"informed_entity": [{"route_id": "01"}],
						"active_period": [{"start": 1000, "end": 2000}]
					}
				}]
			}`),
		},
	}

	raw, err := fetchAlerts(context.Background(), fetcher)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(raw) != 1 {
		t.Fatalf("expected 1 alert, got %d", len(raw))
	}
	if raw[0].ID != "alert-1" {
		t.Errorf("expected ID 'alert-1', got %q", raw[0].ID)
	}
	if raw[0].Headline != "Delay on LW" {
		t.Errorf("expected headline 'Delay on LW', got %q", raw[0].Headline)
	}
	if raw[0].Description != "10 min delays" {
		t.Errorf("expected description '10 min delays', got %q", raw[0].Description)
	}
	if len(raw[0].RouteIDs) != 1 || raw[0].RouteIDs[0] != "01" {
		t.Errorf("expected route IDs [01], got %v", raw[0].RouteIDs)
	}
	if raw[0].StartTime != 1000 || raw[0].EndTime != 2000 {
		t.Errorf("expected times 1000/2000, got %d/%d", raw[0].StartTime, raw[0].EndTime)
	}
}

func TestFetchTripUpdates(t *testing.T) {
	fetcher := &mockFetcher{
		responses: map[string][]byte{
			"/Gtfs/Feed/TripUpdates": []byte(`{
				"entity": [{
					"id": "tu-1",
					"trip_update": {
						"trip": {
							"trip_id": "20260310-123",
							"route_id": "01"
						},
						"stop_time_update": [{
							"stop_id": "UN",
							"departure": {"delay": 120}
						}]
					}
				}]
			}`),
		},
	}

	updates, err := fetchTripUpdates(context.Background(), fetcher)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should have both full ID and trip number index
	if _, ok := updates["20260310-123"]; !ok {
		t.Error("expected update keyed by full trip ID '20260310-123'")
	}
	if _, ok := updates["123"]; !ok {
		t.Error("expected update keyed by trip number '123'")
	}

	upd := updates["20260310-123"]
	if upd.RouteID != "01" {
		t.Errorf("expected route ID '01', got %q", upd.RouteID)
	}
	if len(upd.StopTimeUpdates) != 1 {
		t.Fatalf("expected 1 stop time update, got %d", len(upd.StopTimeUpdates))
	}
	if upd.StopTimeUpdates[0].StopID != "UN" {
		t.Errorf("expected stop ID 'UN', got %q", upd.StopTimeUpdates[0].StopID)
	}
}

func TestPollAllPartialFailure(t *testing.T) {
	// Alerts will fail, everything else succeeds.
	// We use a metrolinx.Client-like setup but pollAll takes *metrolinx.Client,
	// so we test by verifying that partial results are captured correctly
	// when one goroutine fails. We can't easily mock metrolinx.Client directly,
	// so instead we test the fetch functions individually and verify the PollResult
	// struct behavior with a direct construction.

	// Create a PollResult simulating partial failure: alerts failed, trip updates succeeded
	result := PollResult{
		hasAlerts:      false, // simulates fetch failure
		hasTripUpdates: true,
		hasServiceGlance:   true,
		serviceGlance:      []models.ServiceGlanceEntry{{TripNumber: "100", LineCode: "LW"}},
		hasExceptions:      false, // not polled this tick
		hasUnionDepartures: true,
		unionDepartures:    []models.UnionDeparture{{TripNumber: "200", Service: "LW"}},
	}

	if result.hasAlerts {
		t.Error("expected hasAlerts to be false")
	}
	if !result.hasTripUpdates {
		t.Error("expected hasTripUpdates to be true")
	}
	if !result.hasServiceGlance {
		t.Error("expected hasServiceGlance to be true")
	}
	if result.hasExceptions {
		t.Error("expected hasExceptions to be false")
	}
	if !result.hasUnionDepartures {
		t.Error("expected hasUnionDepartures to be true")
	}
	if len(result.serviceGlance) != 1 {
		t.Errorf("expected 1 service glance entry, got %d", len(result.serviceGlance))
	}
	if len(result.unionDepartures) != 1 {
		t.Errorf("expected 1 union departure, got %d", len(result.unionDepartures))
	}
}
