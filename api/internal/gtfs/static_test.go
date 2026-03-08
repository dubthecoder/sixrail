package gtfs_test

import (
	"archive/zip"
	"bytes"
	"testing"
	"time"

	gtfsstore "github.com/dubthecoder/railsix/api/internal/gtfs"
)

// buildTestZip creates a minimal GTFS zip with stops.txt, routes.txt,
// agency.txt, calendar.txt, and trips.txt (required by parser).
func buildTestZip(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := zip.NewWriter(&buf)

	files := map[string]string{
		"agency.txt":   "agency_id,agency_name,agency_url,agency_timezone\nMX,Metrolinx,https://metrolinx.com,America/Toronto\n",
		"calendar.txt": "service_id,monday,tuesday,wednesday,thursday,friday,saturday,sunday,start_date,end_date\nWD,1,1,1,1,1,0,0,20260101,20261231\n",
		"routes.txt":   "route_id,agency_id,route_short_name,route_long_name,route_type,route_color,route_text_color\n01,MX,LW,Lakeshore West,2,098137,FFFFFF\n09,MX,LE,Lakeshore East,2,098137,FFFFFF\n",
		"stops.txt":    "stop_id,stop_code,stop_name,stop_lat,stop_lon,location_type,parent_station\nUN,UN,Union Station,43.6453,-79.3806,1,\nUNp1,UNp1,Union Station Platform 1,43.6454,-79.3807,0,UN\n",
		"trips.txt":      "route_id,service_id,trip_id,direction_id\n01,WD,T001,0\n",
		"stop_times.txt": "trip_id,arrival_time,departure_time,stop_id,stop_sequence\nT001,08:00:00,08:00:00,UN,1\n",
	}

	for name, content := range files {
		f, err := w.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		f.Write([]byte(content))
	}
	w.Close()
	return buf.Bytes()
}

func TestStaticStore_LoadFromBytes(t *testing.T) {
	zipData := buildTestZip(t)
	store, err := gtfsstore.NewStaticStore(zipData)
	if err != nil {
		t.Fatal(err)
	}

	stops := store.AllStops()
	if len(stops) == 0 {
		t.Fatal("expected stops")
	}

	found := false
	for _, s := range stops {
		if s.Code == "UN" {
			found = true
			if s.Name != "Union Station" {
				t.Fatalf("expected Union Station, got %s", s.Name)
			}
		}
	}
	if !found {
		t.Fatal("Union Station not found")
	}
}

func TestStaticStore_GetRoute(t *testing.T) {
	zipData := buildTestZip(t)
	store, err := gtfsstore.NewStaticStore(zipData)
	if err != nil {
		t.Fatal(err)
	}

	r, ok := store.GetRoute("01")
	if !ok {
		t.Fatal("expected route 01")
	}
	if r.ShortName != "LW" {
		t.Fatalf("expected LW, got %s", r.ShortName)
	}
	if r.Color != "098137" {
		t.Fatalf("expected 098137, got %s", r.Color)
	}
}

func TestStaticStore_GetRoute_NotFound(t *testing.T) {
	zipData := buildTestZip(t)
	store, err := gtfsstore.NewStaticStore(zipData)
	if err != nil {
		t.Fatal(err)
	}

	_, ok := store.GetRoute("99")
	if ok {
		t.Fatal("expected route 99 to not be found")
	}
}

func TestStaticStore_IsServiceActive_Weekday(t *testing.T) {
	store, err := gtfsstore.NewStaticStore(buildTestZip(t))
	if err != nil {
		t.Fatal(err)
	}
	tuesday := time.Date(2026, 3, 3, 9, 0, 0, 0, time.UTC)
	if !store.IsServiceActive("WD", tuesday) {
		t.Fatal("expected WD service to be active on Tuesday")
	}
}

func TestStaticStore_IsServiceActive_Weekend(t *testing.T) {
	store, err := gtfsstore.NewStaticStore(buildTestZip(t))
	if err != nil {
		t.Fatal(err)
	}
	saturday := time.Date(2026, 3, 7, 9, 0, 0, 0, time.UTC)
	if store.IsServiceActive("WD", saturday) {
		t.Fatal("expected WD service to not be active on Saturday")
	}
}
