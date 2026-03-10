package main

import (
	"testing"
	"time"

	"github.com/teclara/railsix/shared/models"
)

func TestExtractTripNumber(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"20260424-LW-1731", "1731"},
		{"20260301-BR-100", "100"},
		{"1731", "1731"},
		{"", ""},
		{"no-dash-at-end-", "no-dash-at-end-"},
	}
	for _, tt := range tests {
		got := extractTripNumber(tt.input)
		if got != tt.want {
			t.Errorf("extractTripNumber(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestFormatTime(t *testing.T) {
	loc, _ := time.LoadLocation("America/Toronto")
	tests := []struct {
		input time.Time
		want  string
	}{
		{time.Date(2026, 3, 10, 8, 5, 0, 0, loc), "08:05"},
		{time.Date(2026, 3, 10, 14, 30, 0, 0, loc), "14:30"},
		{time.Date(2026, 3, 10, 0, 0, 0, 0, loc), "00:00"},
		{time.Date(2026, 3, 10, 23, 59, 0, 0, loc), "23:59"},
	}
	for _, tt := range tests {
		got := formatTime(tt.input)
		if got != tt.want {
			t.Errorf("formatTime(%v) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestNetworkHealthComputation(t *testing.T) {
	entries := []models.ServiceGlanceEntry{
		{TripNumber: "100", LineCode: "LW"},
		{TripNumber: "101", LineCode: "LW"},
		{TripNumber: "200", LineCode: "LE"},
		{TripNumber: "300", LineCode: "BR"},
		{TripNumber: "301", LineCode: "BR"},
		{TripNumber: "302", LineCode: "BR"},
		{TripNumber: "400", LineCode: ""},
	}

	counts := make(map[string]int)
	for _, e := range entries {
		if e.LineCode != "" {
			counts[e.LineCode]++
		}
	}

	result := make([]models.NetworkLine, len(allLines))
	for i, l := range allLines {
		result[i] = models.NetworkLine{
			LineCode:    l.code,
			LineName:    l.name,
			ActiveTrips: counts[l.code],
		}
	}

	expected := map[string]int{
		"BR": 3,
		"GT": 0,
		"KI": 0,
		"LE": 1,
		"LW": 2,
		"MI": 0,
		"ST": 0,
	}

	if len(result) != len(allLines) {
		t.Fatalf("expected %d lines, got %d", len(allLines), len(result))
	}
	for _, r := range result {
		if r.ActiveTrips != expected[r.LineCode] {
			t.Errorf("line %s: got %d active trips, want %d", r.LineCode, r.ActiveTrips, expected[r.LineCode])
		}
	}
}

func TestBestNSMatch(t *testing.T) {
	candidates := []models.NextServiceLine{
		{LineCode: "LW", ComputedTime: "08:10"},
		{LineCode: "LW", ComputedTime: "08:30"},
		{LineCode: "LW", ComputedTime: "09:00"},
	}

	tests := []struct {
		scheduled string
		wantIdx   int
		wantNil   bool
	}{
		{"08:05", 0, false},   // closest to 08:10 (5min diff)
		{"08:12", 0, false},   // closest to 08:10 (2min diff)
		{"08:25", 1, false},   // closest to 08:30 (5min diff)
		{"08:45", -1, true},   // 08:30 is 15min away, 09:00 is 15min away — both outside window
		{"09:05", 2, false},   // closest to 09:00 (5min diff)
		{"10:00", -1, true},   // nothing within 10min
		{"invalid", -1, true}, // invalid time
	}

	for _, tt := range tests {
		// Make a copy so removals don't affect subsequent tests.
		cands := make([]models.NextServiceLine, len(candidates))
		copy(cands, candidates)

		ns, idx := bestNSMatch(tt.scheduled, cands)
		if tt.wantNil {
			if ns != nil {
				t.Errorf("bestNSMatch(%q): expected nil, got idx=%d", tt.scheduled, idx)
			}
		} else {
			if ns == nil {
				t.Errorf("bestNSMatch(%q): expected idx=%d, got nil", tt.scheduled, tt.wantIdx)
			} else if idx != tt.wantIdx {
				t.Errorf("bestNSMatch(%q): got idx=%d, want idx=%d", tt.scheduled, idx, tt.wantIdx)
			}
		}
	}
}
