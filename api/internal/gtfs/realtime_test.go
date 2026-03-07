package gtfs_test

import (
	"testing"

	gtfsstore "github.com/teclara/sixrail/api/internal/gtfs"
	"github.com/teclara/sixrail/api/internal/models"
)

type mockStaticLookup struct{}

func (m *mockStaticLookup) GetRoute(id string) (models.Route, bool) {
	if id == "01" {
		return models.Route{ID: "01", ShortName: "LW", LongName: "Lakeshore West", Color: "098137"}, true
	}
	return models.Route{}, false
}

func TestEnrichAlerts(t *testing.T) {
	lookup := &mockStaticLookup{}
	raw := []gtfsstore.RawAlert{
		{ID: "A1", Effect: "REDUCED_SERVICE", Headline: "Delays on LW", Description: "Expect delays", RouteIDs: []string{"01"}},
	}

	enriched := gtfsstore.EnrichAlerts(raw, lookup)
	if len(enriched) != 1 {
		t.Fatalf("expected 1, got %d", len(enriched))
	}
	if enriched[0].RouteNames[0] != "Lakeshore West" {
		t.Fatalf("expected Lakeshore West, got %s", enriched[0].RouteNames[0])
	}
}

func TestRealtimeCache_SetGetAlerts(t *testing.T) {
	rc := gtfsstore.NewRealtimeCache()
	rc.SetAlerts([]models.Alert{
		{ID: "A1", Headline: "Test"},
	})
	alerts := rc.GetAlerts()
	if len(alerts) != 1 || alerts[0].ID != "A1" {
		t.Fatalf("unexpected: %+v", alerts)
	}
}
