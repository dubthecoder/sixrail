package metrolinx

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/teclara/sixrail/api/internal/models"
)

// nextServiceResponse mirrors the Metrolinx NextService JSON structure.
type nextServiceResponse struct {
	NextService struct {
		Lines []struct {
			LineCode               string  `json:"LineCode"`
			LineName               string  `json:"LineName"`
			DirectionName          string  `json:"DirectionName"`
			ScheduledDepartureTime string  `json:"ScheduledDepartureTime"`
			ComputedDepartureTime  string  `json:"ComputedDepartureTime"`
			ScheduledPlatform      string  `json:"ScheduledPlatform"`
			ActualPlatform         string  `json:"ActualPlatform"`
			TripNumber             string  `json:"TripNumber"`
			Status                 string  `json:"Status"`
			Latitude               float64 `json:"Latitude"`
			Longitude              float64 `json:"Longitude"`
		} `json:"Lines"`
	} `json:"NextService"`
}

// unionDeparturesResponse mirrors the Metrolinx UnionDepartures JSON structure.
type unionDeparturesResponse struct {
	AllDepartures struct {
		Trip []struct {
			TripNumber string `json:"TripNumber"`
			Service    string `json:"Service"`
			Platform   string `json:"Platform"`
			Time       string `json:"Time"`
			Info       string `json:"Info"`
			Stops      []struct {
				Name string `json:"Name"`
			} `json:"Stops"`
		} `json:"Trip"`
	} `json:"AllDepartures"`
}

// GetNextService fetches real-time next service for a stop code.
func (c *Client) GetNextService(ctx context.Context, stopCode string) ([]models.NextServiceLine, error) {
	data, err := c.Fetch(ctx, fmt.Sprintf("/Stop/NextService/%s", stopCode))
	if err != nil {
		return nil, err
	}
	var resp nextServiceResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parsing NextService: %w", err)
	}

	lines := make([]models.NextServiceLine, 0, len(resp.NextService.Lines))
	for _, l := range resp.NextService.Lines {
		status := parseStatus(l.Status, l.Latitude, l.Longitude)
		lines = append(lines, models.NextServiceLine{
			LineCode:       l.LineCode,
			LineName:       l.LineName,
			Direction:      l.DirectionName,
			ScheduledTime:  parseMetrolinxTime(l.ScheduledDepartureTime),
			ComputedTime:   parseMetrolinxTime(l.ComputedDepartureTime),
			Platform:       strings.TrimSpace(l.ScheduledPlatform),
			ActualPlatform: strings.TrimSpace(l.ActualPlatform),
			TripNumber:     l.TripNumber,
			Status:         status,
		})
	}
	return lines, nil
}

// GetUnionDepartures fetches the live Union Station departures board.
// mode can be "All", "Train", or "Bus".
func (c *Client) GetUnionDepartures(ctx context.Context, mode string) ([]models.UnionDeparture, error) {
	if mode == "" {
		mode = "All"
	}
	data, err := c.Fetch(ctx, "/ServiceUpdate/UnionDepartures/"+mode)
	if err != nil {
		return nil, err
	}
	var resp unionDeparturesResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parsing UnionDepartures: %w", err)
	}

	deps := make([]models.UnionDeparture, 0, len(resp.AllDepartures.Trip))
	for _, t := range resp.AllDepartures.Trip {
		stops := make([]string, 0, len(t.Stops))
		for _, s := range t.Stops {
			if s.Name != "" {
				stops = append(stops, s.Name)
			}
		}
		// Extract just the keyword from Info (e.g. "Proceed / Avancez" → "PROCEED")
		info := strings.ToUpper(strings.TrimSpace(strings.Split(t.Info, "/")[0]))
		deps = append(deps, models.UnionDeparture{
			TripNumber: t.TripNumber,
			Service:    t.Service,
			Platform:   t.Platform,
			Time:       parseMetrolinxTime(t.Time),
			Info:       info,
			Stops:      stops,
		})
	}
	sort.Slice(deps, func(i, j int) bool {
		return deps[i].Time < deps[j].Time
	})
	return deps, nil
}

// parseMetrolinxTime extracts "HH:MM" from "YYYY-MM-DD HH:MM:SS".
func parseMetrolinxTime(s string) string {
	t, err := time.Parse("2006-01-02 15:04:05", s)
	if err != nil {
		return "--:--"
	}
	return fmt.Sprintf("%02d:%02d", t.Hour(), t.Minute())
}

// parseStatus maps Metrolinx status codes to human-readable strings.
// Status "M" = moving (vehicle has GPS), "S" = scheduled (no GPS fix yet).
func parseStatus(code string, lat, lon float64) string {
	if lat > 0 && lon < 0 {
		return "Moving"
	}
	switch code {
	case "M":
		return "Moving"
	case "S":
		return "On Time"
	default:
		return "On Time"
	}
}
