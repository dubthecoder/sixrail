package metrolinx

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/teclara/railsix/shared/models"
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
			TripNumber  string `json:"TripNumber"`
			Service     string `json:"Service"`
			ServiceType string `json:"ServiceType"`
			Platform    string `json:"Platform"`
			Time        string `json:"Time"`
			Info        string `json:"Info"`
			Stops       []struct {
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
func (c *Client) GetUnionDepartures(ctx context.Context) ([]models.UnionDeparture, error) {
	data, err := c.Fetch(ctx, "/ServiceUpdate/UnionDepartures/All")
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
			TripNumber:  t.TripNumber,
			Service:     t.Service,
			ServiceType: t.ServiceType,
			Platform:    t.Platform,
			Time:        parseMetrolinxTime(t.Time),
			Info:        info,
			Stops:       stops,
		})
	}
	// Sort by time, treating times before 06:00 as next-day for midnight-crossing schedules.
	sort.Slice(deps, func(i, j int) bool {
		return sortableTime(deps[i].Time) < sortableTime(deps[j].Time)
	})
	return deps, nil
}

// serviceGlanceResponse mirrors the Metrolinx ServiceataGlance/Trains/All JSON structure.
type serviceGlanceResponse struct {
	Trips struct {
		Trip []struct {
			Cars                string  `json:"Cars"`
			TripNumber          string  `json:"TripNumber"`
			LineCode            string  `json:"LineCode"`
			Display             string  `json:"Display"`
			DelaySeconds        int     `json:"DelaySeconds"`
			OccupancyPercentage int     `json:"OccupancyPercentage"`
			Latitude            float64 `json:"Latitude"`
			Longitude           float64 `json:"Longitude"`
			IsInMotion          bool    `json:"IsInMotion"`
		} `json:"Trip"`
	} `json:"Trips"`
}

// exceptionsResponse mirrors the Metrolinx ServiceUpdate/Exceptions/All JSON structure.
type exceptionsResponse struct {
	Trip []struct {
		TripNumber  string `json:"TripNumber"`
		IsCancelled string `json:"IsCancelled"`
		Stop        []struct {
			Code        string `json:"Code"`
			IsCancelled string `json:"IsCancelled"`
		} `json:"Stop"`
	} `json:"Trip"`
}


// GetServiceGlance fetches all in-service train trips with occupancy and car count.
func (c *Client) GetServiceGlance(ctx context.Context) ([]models.ServiceGlanceEntry, error) {
	data, err := c.Fetch(ctx, "/ServiceataGlance/Trains/All")
	if err != nil {
		return nil, err
	}
	var resp serviceGlanceResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parsing ServiceGlance: %w", err)
	}

	entries := make([]models.ServiceGlanceEntry, 0, len(resp.Trips.Trip))
	for _, t := range resp.Trips.Trip {
		entries = append(entries, models.ServiceGlanceEntry{
			TripNumber:   t.TripNumber,
			LineCode:     t.LineCode,
			LineName:     strings.TrimSpace(t.Display),
			Cars:         t.Cars,
			DelaySeconds: t.DelaySeconds,
			Lat:          t.Latitude,
			Lon:          t.Longitude,
			IsInMotion:   t.IsInMotion,
		})
	}
	return entries, nil
}

// GetExceptions fetches cancelled trips and returns a map of trip number to cancelled stop codes.
// An empty slice means the whole trip is cancelled; a non-empty slice lists specific cancelled stops.
func (c *Client) GetExceptions(ctx context.Context) (map[string][]string, error) {
	data, err := c.Fetch(ctx, "/ServiceUpdate/Exceptions/All")
	if err != nil {
		return nil, err
	}
	var resp exceptionsResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("parsing Exceptions: %w", err)
	}

	exceptions := make(map[string][]string)
	for _, t := range resp.Trip {
		if t.IsCancelled == "1" || strings.EqualFold(t.IsCancelled, "true") {
			exceptions[t.TripNumber] = []string{}
			continue
		}
		var cancelledStops []string
		for _, s := range t.Stop {
			if s.IsCancelled == "1" || strings.EqualFold(s.IsCancelled, "true") {
				cancelledStops = append(cancelledStops, s.Code)
			}
		}
		if len(cancelledStops) > 0 {
			exceptions[t.TripNumber] = cancelledStops
		}
	}
	return exceptions, nil
}

// sortableTime returns a string that sorts correctly across midnight.
// Times before "06:00" are treated as next-day to keep late-night trains in order.
func sortableTime(t string) string {
	if len(t) >= 2 && t < "06:" {
		return "1" + t // push after "23:xx"
	}
	return "0" + t
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
// GPS coordinates are used as a fallback when the status code is absent.
func parseStatus(code string, lat, lon float64) string {
	switch code {
	case "M":
		return "Moving"
	case "S":
		return "On Time"
	default:
		// No status code — fall back to GPS coordinates.
		if lat > 0 && lon < 0 {
			return "Moving"
		}
		return "On Time"
	}
}
