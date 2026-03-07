package models

import (
	"encoding/json"
	"testing"
)

func TestStopRoundTrip(t *testing.T) {
	original := Stop{
		ID:       "UN",
		Code:     "00100",
		Name:     "Union Station",
		Lat:      43.6453,
		Lon:      -79.3806,
		ParentID: "UN_PARENT",
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded Stop
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded != original {
		t.Errorf("round-trip mismatch:\n got  %+v\n want %+v", decoded, original)
	}
}

func TestStopOmitEmptyParentID(t *testing.T) {
	s := Stop{ID: "UN", Code: "00100", Name: "Union Station", Lat: 43.6, Lon: -79.3}

	data, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal to map: %v", err)
	}

	if _, exists := raw["parentId"]; exists {
		t.Error("expected parentId to be omitted when empty")
	}
}

func TestStopParentIDPresent(t *testing.T) {
	s := Stop{ID: "UN", Code: "00100", Name: "Union Station", Lat: 43.6, Lon: -79.3, ParentID: "P1"}

	data, err := json.Marshal(s)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal to map: %v", err)
	}

	if _, exists := raw["parentId"]; !exists {
		t.Error("expected parentId to be present when set")
	}
}

func TestRouteRoundTrip(t *testing.T) {
	original := Route{
		ID:        "01",
		ShortName: "LW",
		LongName:  "Lakeshore West",
		Color:     "00853F",
		TextColor: "FFFFFF",
		Type:      2,
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded Route
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded != original {
		t.Errorf("round-trip mismatch:\n got  %+v\n want %+v", decoded, original)
	}
}

func TestRouteTypeZeroNotOmitted(t *testing.T) {
	r := Route{ID: "01", ShortName: "LW", LongName: "Lakeshore West", Color: "00853F", TextColor: "FFFFFF", Type: 0}

	data, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal to map: %v", err)
	}

	if _, exists := raw["type"]; !exists {
		t.Error("expected type to be present even when zero")
	}
}

func TestAlertRoundTrip(t *testing.T) {
	original := Alert{
		ID:          "A001",
		Effect:      "DETOUR",
		Headline:    "Service disruption",
		Description: "Track work between Union and Bloor.",
		URL:         "https://gotransit.com/alerts/A001",
		RouteIDs:    []string{"01", "02"},
		RouteNames:  []string{"Lakeshore West", "Lakeshore East"},
		StartTime:   1709500000,
		EndTime:     1709600000,
	}

	data, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var decoded Alert
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if decoded.ID != original.ID ||
		decoded.Effect != original.Effect ||
		decoded.Headline != original.Headline ||
		decoded.Description != original.Description ||
		decoded.URL != original.URL ||
		decoded.StartTime != original.StartTime ||
		decoded.EndTime != original.EndTime ||
		len(decoded.RouteIDs) != len(original.RouteIDs) ||
		len(decoded.RouteNames) != len(original.RouteNames) {
		t.Errorf("round-trip mismatch:\n got  %+v\n want %+v", decoded, original)
	}

	for i, id := range original.RouteIDs {
		if decoded.RouteIDs[i] != id {
			t.Errorf("RouteIDs[%d] = %q, want %q", i, decoded.RouteIDs[i], id)
		}
	}
	for i, name := range original.RouteNames {
		if decoded.RouteNames[i] != name {
			t.Errorf("RouteNames[%d] = %q, want %q", i, decoded.RouteNames[i], name)
		}
	}
}

func TestAlertOmitEmptyOptionalFields(t *testing.T) {
	a := Alert{
		ID:          "A001",
		Effect:      "DETOUR",
		Headline:    "Service disruption",
		Description: "Track work.",
	}

	data, err := json.Marshal(a)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal to map: %v", err)
	}

	for _, key := range []string{"url", "routeIds", "routeNames", "startTime", "endTime"} {
		if _, exists := raw[key]; exists {
			t.Errorf("expected %q to be omitted when empty/zero", key)
		}
	}
}

func TestAlertOptionalFieldsPresent(t *testing.T) {
	a := Alert{
		ID:          "A001",
		Effect:      "DETOUR",
		Headline:    "Service disruption",
		Description: "Track work.",
		URL:         "https://example.com",
		RouteIDs:    []string{"01"},
		RouteNames:  []string{"Lakeshore West"},
		StartTime:   1709500000,
		EndTime:     1709600000,
	}

	data, err := json.Marshal(a)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal to map: %v", err)
	}

	for _, key := range []string{"url", "routeIds", "routeNames", "startTime", "endTime"} {
		if _, exists := raw[key]; !exists {
			t.Errorf("expected %q to be present when set", key)
		}
	}
}

func TestStopFromJSON(t *testing.T) {
	input := `{"id":"UN","code":"00100","name":"Union Station","lat":43.6453,"lon":-79.3806}`

	var s Stop
	if err := json.Unmarshal([]byte(input), &s); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if s.ID != "UN" || s.Name != "Union Station" || s.ParentID != "" {
		t.Errorf("unexpected values: %+v", s)
	}
}

