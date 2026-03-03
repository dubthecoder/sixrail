package gtfs_test

import (
	"testing"
	"time"

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

func TestEnrichPositions(t *testing.T) {
	lookup := &mockStaticLookup{}
	positions := []gtfsstore.RawPosition{
		{VehicleID: "V1", TripID: "T001", RouteID: "01", Lat: 43.65, Lon: -79.38, Bearing: 180, Speed: 50, Timestamp: time.Now().Unix()},
		{VehicleID: "V2", TripID: "T002", RouteID: "99", Lat: 43.66, Lon: -79.39, Timestamp: time.Now().Unix()},
	}

	enriched := gtfsstore.EnrichPositions(positions, lookup)
	if len(enriched) != 2 {
		t.Fatalf("expected 2, got %d", len(enriched))
	}
	if enriched[0].RouteName != "Lakeshore West" {
		t.Fatalf("expected Lakeshore West, got %s", enriched[0].RouteName)
	}
	if enriched[0].RouteColor != "098137" {
		t.Fatalf("expected 098137, got %s", enriched[0].RouteColor)
	}
	if enriched[1].RouteName != "" {
		t.Fatalf("expected empty route name for unknown route, got %s", enriched[1].RouteName)
	}
}

func TestEnrichAlerts(t *testing.T) {
	lookup := &mockStaticLookup{}
	raw := []gtfsstore.RawAlert{
		{ID: "A1", Effect: "REDUCED_SERVICE", Headline: "Delays on LW", Description: "Expect delays", RouteIDs: []string{"01"}, StartTime: time.Now().Unix()},
	}

	enriched := gtfsstore.EnrichAlerts(raw, lookup)
	if len(enriched) != 1 {
		t.Fatalf("expected 1, got %d", len(enriched))
	}
	if enriched[0].RouteNames[0] != "Lakeshore West" {
		t.Fatalf("expected Lakeshore West, got %s", enriched[0].RouteNames[0])
	}
}

func TestRealtimeCache_SetGetPositions(t *testing.T) {
	rc := gtfsstore.NewRealtimeCache()
	rc.SetPositions([]models.VehiclePosition{
		{VehicleID: "V1", Lat: 43.65, Lon: -79.38},
	})
	positions := rc.GetPositions()
	if len(positions) != 1 || positions[0].VehicleID != "V1" {
		t.Fatalf("unexpected: %+v", positions)
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
