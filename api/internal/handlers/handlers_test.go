package handlers_test

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/teclara/gopulse/api/internal/cache"
	gtfsstore "github.com/teclara/gopulse/api/internal/gtfs"
	"github.com/teclara/gopulse/api/internal/handlers"
	"github.com/teclara/gopulse/api/internal/models"
)

type mockFetcher struct {
	response []byte
	err      error
}

func (m *mockFetcher) Fetch(ctx context.Context, path string) ([]byte, error) {
	return m.response, m.err
}

func buildTestZip(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)
	files := map[string]string{
		"agency.txt":     "agency_id,agency_name,agency_url,agency_timezone\nMX,Metrolinx,https://metrolinx.com,America/Toronto\n",
		"calendar.txt":   "service_id,monday,tuesday,wednesday,thursday,friday,saturday,sunday,start_date,end_date\nWD,1,1,1,1,1,0,0,20260101,20261231\n",
		"routes.txt":     "route_id,agency_id,route_short_name,route_long_name,route_type,route_color,route_text_color\n01,MX,LW,Lakeshore West,2,098137,FFFFFF\n",
		"stops.txt":      "stop_id,stop_code,stop_name,stop_lat,stop_lon,location_type,parent_station\nUN,UN,Union Station,43.6453,-79.3806,1,\n",
		"trips.txt":      "route_id,service_id,trip_id,direction_id\n01,WD,T001,0\n",
		"stop_times.txt": "trip_id,arrival_time,departure_time,stop_id,stop_sequence\nT001,08:00:00,08:00:00,UN,1\n",
	}
	for name, content := range files {
		f, _ := w.Create(name)
		f.Write([]byte(content))
	}
	w.Close()
	return buf.Bytes()
}

func mustBuildStore(t *testing.T) *gtfsstore.StaticStore {
	t.Helper()
	store, err := gtfsstore.NewStaticStore(buildTestZip(t))
	if err != nil {
		t.Fatal(err)
	}
	return store
}

func TestHealthHandler(t *testing.T) {
	h := handlers.New(nil, nil, nil, nil)
	req := httptest.NewRequest("GET", "/api/health", nil)
	w := httptest.NewRecorder()

	h.Health(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var body map[string]string
	json.Unmarshal(w.Body.Bytes(), &body)
	if body["status"] != "ok" {
		t.Fatalf("expected ok, got %s", body["status"])
	}
}

func TestAllStops(t *testing.T) {
	store := mustBuildStore(t)
	h := handlers.New(nil, nil, store, nil)

	req := httptest.NewRequest("GET", "/api/stops", nil)
	w := httptest.NewRecorder()
	h.AllStops(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var stops []models.Stop
	if err := json.Unmarshal(w.Body.Bytes(), &stops); err != nil {
		t.Fatalf("failed to unmarshal stops: %v", err)
	}
	if len(stops) == 0 {
		t.Fatal("expected at least one stop")
	}
	found := false
	for _, s := range stops {
		if s.ID == "UN" && s.Name == "Union Station" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected Union Station stop in response")
	}
}

func TestPositions(t *testing.T) {
	rt := gtfsstore.NewRealtimeCache()
	rt.SetPositions([]models.VehiclePosition{
		{VehicleID: "V1", TripID: "T1", RouteID: "01", Lat: 43.6, Lon: -79.3, Timestamp: 1000},
	})

	h := handlers.New(nil, nil, nil, rt)
	req := httptest.NewRequest("GET", "/api/positions", nil)
	w := httptest.NewRecorder()
	h.Positions(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var positions []models.VehiclePosition
	if err := json.Unmarshal(w.Body.Bytes(), &positions); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if len(positions) != 1 || positions[0].VehicleID != "V1" {
		t.Fatalf("unexpected positions: %+v", positions)
	}
}

func TestPositions_Empty(t *testing.T) {
	rt := gtfsstore.NewRealtimeCache()
	h := handlers.New(nil, nil, nil, rt)

	req := httptest.NewRequest("GET", "/api/positions", nil)
	w := httptest.NewRecorder()
	h.Positions(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if w.Body.String() != "[]" {
		t.Fatalf("expected empty array, got %s", w.Body.String())
	}
}

func TestAlerts(t *testing.T) {
	rt := gtfsstore.NewRealtimeCache()
	rt.SetAlerts([]models.Alert{
		{ID: "A1", Headline: "Delay on LW", Effect: "DELAY"},
	})

	h := handlers.New(nil, nil, nil, rt)
	req := httptest.NewRequest("GET", "/api/alerts", nil)
	w := httptest.NewRecorder()
	h.Alerts(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var alerts []models.Alert
	if err := json.Unmarshal(w.Body.Bytes(), &alerts); err != nil {
		t.Fatalf("failed to unmarshal: %v", err)
	}
	if len(alerts) != 1 || alerts[0].ID != "A1" {
		t.Fatalf("unexpected alerts: %+v", alerts)
	}
}

func TestAlerts_Empty(t *testing.T) {
	rt := gtfsstore.NewRealtimeCache()
	h := handlers.New(nil, nil, nil, rt)

	req := httptest.NewRequest("GET", "/api/alerts", nil)
	w := httptest.NewRecorder()
	h.Alerts(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if w.Body.String() != "[]" {
		t.Fatalf("expected empty array, got %s", w.Body.String())
	}
}

func TestStopDepartures_CacheHit(t *testing.T) {
	c := cache.New()
	c.Set("/Stop/NextService/UN", []byte(`{"departures":[]}`), 30*time.Second)

	fetcher := &mockFetcher{response: []byte(`should not be called`)}
	h := handlers.New(fetcher, c, nil, nil)

	req := httptest.NewRequest("GET", "/api/departures/UN", nil)
	req.SetPathValue("stopCode", "UN")
	w := httptest.NewRecorder()
	h.StopDepartures(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if w.Header().Get("X-Cache") != "HIT" {
		t.Fatalf("expected X-Cache HIT, got %s", w.Header().Get("X-Cache"))
	}
	if w.Body.String() != `{"departures":[]}` {
		t.Fatalf("expected cached data, got %s", w.Body.String())
	}
}

func TestStopDepartures_CacheMiss(t *testing.T) {
	c := cache.New()
	fetcher := &mockFetcher{response: []byte(`{"departures":[{"trip":"123"}]}`)}
	h := handlers.New(fetcher, c, nil, nil)

	req := httptest.NewRequest("GET", "/api/departures/UN", nil)
	req.SetPathValue("stopCode", "UN")
	w := httptest.NewRecorder()
	h.StopDepartures(w, req)

	if w.Code != 200 {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	if w.Header().Get("X-Cache") != "MISS" {
		t.Fatalf("expected X-Cache MISS, got %s", w.Header().Get("X-Cache"))
	}
	if w.Body.String() != `{"departures":[{"trip":"123"}]}` {
		t.Fatalf("unexpected body: %s", w.Body.String())
	}

	// verify it was cached
	val, ok := c.Get("/Stop/NextService/UN")
	if !ok {
		t.Fatal("expected value to be cached")
	}
	if string(val) != `{"departures":[{"trip":"123"}]}` {
		t.Fatalf("unexpected cached value: %s", string(val))
	}
}

func TestStopDepartures_InvalidCode(t *testing.T) {
	h := handlers.New(nil, nil, nil, nil)

	cases := []string{"../etc", "", "A", "TOOLONGSTOPCODE123"}
	for _, code := range cases {
		req := httptest.NewRequest("GET", "/api/departures/"+code, nil)
		req.SetPathValue("stopCode", code)
		w := httptest.NewRecorder()
		h.StopDepartures(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("code %q: expected 400, got %d", code, w.Code)
		}
	}
}
