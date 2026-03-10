package gtfsrt

import (
	"testing"

	"github.com/teclara/railsix/shared/models"
)

func TestParseAlertsEmpty(t *testing.T) {
	data := []byte(`{"entity":[]}`)
	alerts, err := ParseAlerts(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(alerts) != 0 {
		t.Fatalf("expected 0 alerts, got %d", len(alerts))
	}
}

func TestParseTripUpdatesEmpty(t *testing.T) {
	data := []byte(`{"entity":[]}`)
	updates, err := ParseTripUpdates(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(updates) != 0 {
		t.Fatalf("expected 0 updates, got %d", len(updates))
	}
}

func TestParseAlertsWithData(t *testing.T) {
	data := []byte(`{
		"entity": [{
			"id": "alert-1",
			"alert": {
				"effect": "REDUCED_SERVICE",
				"header_text": {
					"translation": [{"text": "Test Alert", "language": "en"}]
				},
				"description_text": {
					"translation": [{"text": "Details here", "language": "en"}]
				},
				"informed_entity": [
					{"route_id": "01"},
					{"route_id": "02"},
					{"route_id": "01"}
				],
				"active_period": [{"start": 1000, "end": 2000}]
			}
		}]
	}`)

	alerts, err := ParseAlerts(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(alerts) != 1 {
		t.Fatalf("expected 1 alert, got %d", len(alerts))
	}

	a := alerts[0]
	if a.ID != "alert-1" {
		t.Errorf("expected ID 'alert-1', got %q", a.ID)
	}
	if a.Headline != "Test Alert" {
		t.Errorf("expected headline 'Test Alert', got %q", a.Headline)
	}
	if a.Description != "Details here" {
		t.Errorf("expected description 'Details here', got %q", a.Description)
	}
	if len(a.RouteIDs) != 2 {
		t.Fatalf("expected 2 route IDs (deduped), got %d", len(a.RouteIDs))
	}
	if a.RouteIDs[0] != "01" || a.RouteIDs[1] != "02" {
		t.Errorf("expected route IDs [01, 02], got %v", a.RouteIDs)
	}
	if a.StartTime != 1000 || a.EndTime != 2000 {
		t.Errorf("expected times 1000/2000, got %d/%d", a.StartTime, a.EndTime)
	}
}

type mockRouteLookup struct {
	routes map[string]models.Route
}

func (m *mockRouteLookup) GetRoute(id string) (models.Route, bool) {
	r, ok := m.routes[id]
	return r, ok
}

func TestEnrichAlerts(t *testing.T) {
	lookup := &mockRouteLookup{
		routes: map[string]models.Route{
			"01": {ID: "01", LongName: "Lakeshore West"},
			"02": {ID: "02", LongName: "Lakeshore East"},
		},
	}

	raw := []RawAlert{
		{
			ID:       "a1",
			Headline: "Delays",
			RouteIDs: []string{"01", "02", "99"},
		},
	}

	enriched := EnrichAlerts(raw, lookup)
	if len(enriched) != 1 {
		t.Fatalf("expected 1 alert, got %d", len(enriched))
	}

	a := enriched[0]
	if a.ID != "a1" {
		t.Errorf("expected ID 'a1', got %q", a.ID)
	}
	if a.Headline != "Delays" {
		t.Errorf("expected headline 'Delays', got %q", a.Headline)
	}
	if len(a.RouteNames) != 2 {
		t.Fatalf("expected 2 route names (99 not found), got %d: %v", len(a.RouteNames), a.RouteNames)
	}
	if a.RouteNames[0] != "Lakeshore West" || a.RouteNames[1] != "Lakeshore East" {
		t.Errorf("expected [Lakeshore West, Lakeshore East], got %v", a.RouteNames)
	}
}
