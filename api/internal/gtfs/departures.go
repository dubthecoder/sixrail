package gtfs

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/teclara/sixrail/api/internal/models"
)

const (
	torontoTZ    = "America/Toronto"
	maxDepartures = 20
	lookAheadHours = 3 // hours of departures to return
)

// GetDepartures returns upcoming departures for a stop code, merging static schedule
// with real-time trip updates. Falls back gracefully if updates are unavailable.
// If destCode is non-empty, ArrivalTime is populated for each departure.
func GetDepartures(stopCode, destCode string, now time.Time, static *StaticStore, rt *RealtimeCache) []models.Departure {
	loc, err := time.LoadLocation(torontoTZ)
	if err != nil {
		loc = time.UTC
	}
	nowLocal := now.In(loc)

	stopIDs := static.StopIDsForCode(stopCode)
	if len(stopIDs) == 0 {
		return []models.Departure{}
	}

	var destStopIDs []string
	if destCode != "" {
		destStopIDs = static.StopIDsForCode(destCode)
	}

	// Determine active service IDs for today (and yesterday for past-midnight trips).
	today := truncateToDay(nowLocal)
	yesterday := today.Add(-24 * time.Hour)

	type candidate struct {
		dep        ScheduledDeparture
		stopID     string    // which platform/stop this departure is from
		serviceDay time.Time // the service day this departure belongs to
		adjusted   time.Time // wall-clock departure time after real-time delay
	}

	var candidates []candidate

	for _, stopID := range stopIDs {
		departures := static.DeparturesForStop(stopID)
		for _, dep := range departures {
			// Try both today and yesterday (for past-midnight services).
			for _, serviceDay := range []time.Time{today, yesterday} {
				if !static.IsServiceActive(dep.ServiceID, serviceDay) {
					continue
				}
				// Compute the wall-clock scheduled departure time.
				// DepartureTime is a duration from midnight of the service day.
				scheduled := serviceDay.Add(dep.DepartureTime)

				// Apply real-time delay if available.
				delay := findDelay(dep.TripID, stopID, rt)
				adjusted := scheduled.Add(delay)

				// Only include if within the look-ahead window and not in the past.
				if adjusted.Before(nowLocal) {
					continue
				}
				if adjusted.After(nowLocal.Add(lookAheadHours * time.Hour)) {
					continue
				}

				candidates = append(candidates, candidate{dep, stopID, serviceDay, adjusted})
				break // matched a service day — no need to check the other
			}
		}
	}

	// Sort by adjusted departure time.
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].adjusted.Before(candidates[j].adjusted)
	})

	// Deduplicate by trip number AND by departure time + line.
	// Same physical train can have multiple GTFS trip IDs due to:
	// 1. Different service date prefixes (e.g. 20260301-LW-1731 vs 20260424-LW-1731)
	// 2. Overlapping service calendars producing different trip numbers for the same departure
	seenTrip := make(map[string]bool)
	seenTimeLine := make(map[string]bool)
	result := make([]models.Departure, 0, maxDepartures)
	for _, c := range candidates {
		tripNum := extractTripNumber(c.dep.TripID)
		if seenTrip[tripNum] {
			continue
		}
		seenTrip[tripNum] = true

		// Also dedup by scheduled time + route + stop: if two different trip numbers
		// produce the same departure time on the same route at the same platform,
		// keep only the first. Different platforms are preserved as distinct departures.
		timeLineKey := formatTime(c.serviceDay.Add(c.dep.DepartureTime)) + "|" + c.dep.RouteID + "|" + c.stopID
		if seenTimeLine[timeLineKey] {
			continue
		}
		seenTimeLine[timeLineKey] = true

		route, _ := static.GetRoute(c.dep.RouteID)
		delay := c.adjusted.Sub(c.serviceDay.Add(c.dep.DepartureTime))
		delayMin := int(delay.Minutes())

		status := "On Time"
		if update, ok := rt.GetTripUpdate(c.dep.TripID); ok {
			if update.ScheduleRelationship == "CANCELED" {
				status = "Cancelled"
			} else if delayMin >= 1 {
				status = fmt.Sprintf("Delayed +%dm", delayMin)
			}
		}

		dep := models.Departure{
			Line:          route.ShortName,
			LineName:      route.LongName,
			Destination:   c.dep.Headsign,
			ScheduledTime: formatTime(c.serviceDay.Add(c.dep.DepartureTime)),
			Status:        status,
			RouteColor:    route.Color,
			DelayMinutes:  delayMin,
			Stops:         static.RemainingStopNames(c.dep.TripID, stopIDs),
			RouteType:     route.Type,
		}
		if len(destStopIDs) > 0 {
			arrDur, ok := static.ArrivalTimeAtStop(c.dep.TripID, destStopIDs, stopIDs...)
			if !ok {
				continue // skip trips that don't stop at the destination (or destination is before origin)
			}
			dep.ArrivalTime = formatTime(c.serviceDay.Add(arrDur))
		}

		// Enrich with service glance data (occupancy, car count).
		// GTFS trip ID format: "20260424-LW-1731" → trip number is "1731"
		tripNumber := extractTripNumber(c.dep.TripID)
		if sg, ok := rt.GetServiceGlanceEntry(tripNumber); ok {
			dep.Cars = sg.Cars
			dep.IsInMotion = sg.IsInMotion
		}
		if occStatus := rt.GetOccupancyStatus(c.dep.TripID); occStatus != "" {
			dep.Occupancy = occStatus
		}

		// Flag cancelled trips from exceptions cache.
		if rt.IsTripCancelled(tripNumber) {
			dep.IsCancelled = true
			dep.Status = "Cancelled"
		}

		result = append(result, dep)

		if len(result) >= maxDepartures {
			break
		}
	}

	return result
}

// findDelay returns the departure delay for a trip at a given stop.
// Returns zero if no update exists.
func findDelay(tripID, stopID string, rt *RealtimeCache) time.Duration {
	update, ok := rt.GetTripUpdate(tripID)
	if !ok {
		return 0
	}
	// Walk stop time updates; last matching stop wins (propagation).
	var delay time.Duration
	for _, stu := range update.StopTimeUpdates {
		if stu.StopID == stopID {
			delay = stu.DepartureDelay
			break
		}
	}
	return delay
}

// extractTripNumber returns the Metrolinx trip number from a GTFS trip ID.
// GTFS trip IDs have the format "20260424-LW-1731"; the trip number is the last segment.
func extractTripNumber(tripID string) string {
	if idx := strings.LastIndex(tripID, "-"); idx >= 0 && idx+1 < len(tripID) {
		return tripID[idx+1:]
	}
	return tripID
}

// formatTime returns "HH:MM" in local time.
func formatTime(t time.Time) string {
	return fmt.Sprintf("%02d:%02d", t.Hour(), t.Minute())
}
