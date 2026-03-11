package gtfsrt

import (
	"encoding/json"
	"time"

	"github.com/teclara/railsix/shared/models"
)

// ParseAlerts parses the Metrolinx GTFS-RT JSON alerts feed.
func ParseAlerts(data []byte) ([]RawAlert, error) {
	var feed gtfsRTFeed
	if err := json.Unmarshal(data, &feed); err != nil {
		return nil, err
	}

	alerts := make([]RawAlert, 0, len(feed.Entity))
	for _, e := range feed.Entity {
		if e.Alert == nil {
			continue
		}
		a := e.Alert

		var headline, description, url string
		if a.HeaderText != nil {
			headline = englishText(a.HeaderText.Translation)
		}
		if a.DescriptionTxt != nil {
			description = englishText(a.DescriptionTxt.Translation)
		}
		if a.URL != nil {
			url = englishText(a.URL.Translation)
		}

		seenRoute := make(map[string]bool)
		seenStop := make(map[string]bool)
		var routeIDs, stopIDs []string
		for _, ie := range a.InformedEntity {
			if ie.RouteID != "" && !seenRoute[ie.RouteID] {
				routeIDs = append(routeIDs, ie.RouteID)
				seenRoute[ie.RouteID] = true
			}
			if ie.StopID != "" && !seenStop[ie.StopID] {
				stopIDs = append(stopIDs, ie.StopID)
				seenStop[ie.StopID] = true
			}
		}

		var startTime, endTime int64
		if len(a.ActivePeriod) > 0 {
			startTime = a.ActivePeriod[0].Start
			endTime = a.ActivePeriod[0].End
		}

		alerts = append(alerts, RawAlert{
			ID:          e.ID,
			Effect:      a.Effect,
			Headline:    headline,
			Description: description,
			URL:         url,
			RouteIDs:    routeIDs,
			StopIDs:     stopIDs,
			StartTime:   startTime,
			EndTime:     endTime,
		})
	}
	return alerts, nil
}

// ParseTripUpdates parses the Metrolinx GTFS-RT JSON trip updates feed.
func ParseTripUpdates(data []byte) (map[string]RawTripUpdate, error) {
	var feed gtfsRTFeed
	if err := json.Unmarshal(data, &feed); err != nil {
		return nil, err
	}

	updates := make(map[string]RawTripUpdate, len(feed.Entity))
	for _, e := range feed.Entity {
		if e.TripUpdate == nil {
			continue
		}
		tu := e.TripUpdate
		tripID := tu.Trip.TripID
		if tripID == "" {
			continue
		}

		raw := RawTripUpdate{
			TripID:               tripID,
			RouteID:              tu.Trip.RouteID,
			ScheduleRelationship: tu.Trip.ScheduleRelationship,
		}

		for _, stu := range tu.StopTimeUpdate {
			var arrDelay, depDelay time.Duration
			if stu.Arrival != nil {
				arrDelay = time.Duration(stu.Arrival.Delay) * time.Second
			}
			if stu.Departure != nil {
				depDelay = time.Duration(stu.Departure.Delay) * time.Second
			}
			raw.StopTimeUpdates = append(raw.StopTimeUpdates, RawStopTimeUpdate{
				StopID:               stu.StopID,
				ArrivalDelay:         arrDelay,
				DepartureDelay:       depDelay,
				ScheduleRelationship: stu.ScheduleRelationship,
			})
		}

		updates[tripID] = raw
		// Also index by trip number (last segment) so lookups work
		// regardless of which date-prefix the static schedule uses.
		tripNum := models.ExtractTripNumber(tripID)
		if tripNum != tripID {
			if _, exists := updates[tripNum]; !exists {
				updates[tripNum] = raw
			}
		}
	}
	return updates, nil
}

// EnrichAlerts converts raw alerts to models.Alert, resolving route IDs to display names.
func EnrichAlerts(raw []RawAlert, lookup RouteLookup) []models.Alert {
	out := make([]models.Alert, len(raw))
	for i, ra := range raw {
		alert := models.Alert{
			ID:          ra.ID,
			Effect:      ra.Effect,
			Headline:    ra.Headline,
			Description: ra.Description,
			URL:         ra.URL,
			RouteIDs:    ra.RouteIDs,
			StopIDs:     ra.StopIDs,
			StartTime:   ra.StartTime,
			EndTime:     ra.EndTime,
		}
		names := make([]string, 0, len(ra.RouteIDs))
		for _, rid := range ra.RouteIDs {
			if route, ok := lookup.GetRoute(rid); ok {
				names = append(names, route.LongName)
			}
		}
		alert.RouteNames = names
		out[i] = alert
	}
	return out
}

func englishText(translations []gtfsRTTranslation) string {
	for _, t := range translations {
		if t.Language == "en" {
			return t.Text
		}
	}
	if len(translations) > 0 {
		return translations[0].Text
	}
	return ""
}
