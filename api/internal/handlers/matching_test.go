package handlers

import (
	"testing"

	"github.com/teclara/railsix/api/internal/models"
)

func TestBestNSMatch_ReturnsClosestCandidateWithinWindow(t *testing.T) {
	candidates := []models.NextServiceLine{
		{ComputedTime: "08:17"},
		{ComputedTime: "08:12"},
		{ComputedTime: "08:30"},
	}

	match, idx := bestNSMatch("08:10", candidates)
	if match == nil {
		t.Fatal("expected a match")
	}
	if idx != 1 {
		t.Fatalf("expected closest candidate index 1, got %d", idx)
	}
	if match.ComputedTime != "08:12" {
		t.Fatalf("expected closest computed time 08:12, got %s", match.ComputedTime)
	}
}

func TestBestNSMatch_RejectsCandidatesOutsideWindow(t *testing.T) {
	candidates := []models.NextServiceLine{
		{ComputedTime: "08:25"},
		{ComputedTime: "07:58"},
	}

	match, idx := bestNSMatch("08:10", candidates)
	if match != nil || idx != -1 {
		t.Fatalf("expected no match, got %+v at index %d", match, idx)
	}
}
