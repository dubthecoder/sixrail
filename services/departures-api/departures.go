package main

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/teclara/railsix/shared/gtfsrt"
	"github.com/teclara/railsix/shared/models"
)

var torontoLoc *time.Location

func init() {
	var err error
	torontoLoc, err = time.LoadLocation("America/Toronto")
	if err != nil {
		panic("failed to load America/Toronto timezone: " + err.Error())
	}
}

// GetDepartures returns upcoming departures for a stop code, merging static schedule
// with real-time trip updates. Falls back gracefully if updates are unavailable.
// If destCode is non-empty, ArrivalTime is populated for each departure.
func GetDepartures(ctx context.Context, stopCode, destCode string, now time.Time, sc *StaticClient, rc *RedisClient) []models.Departure {
	nowLocal := now.In(torontoLoc)

	// Single bulk call to gtfs-static — all filtering done server-side.
	candidates, err := sc.GetSchedule(stopCode, nowLocal)
	if err != nil {
		slog.Warn("failed to get schedule", "stopCode", stopCode, "error", err)
		return []models.Departure{}
	}
	if len(candidates) == 0 {
		return []models.Departure{}
	}

	// Destination filtering needs stop IDs for arrival time lookup.
	var destStopIDs []string
	if destCode != "" {
		destStopIDs, err = sc.StopIDsForCode(destCode)
		if err != nil {
			slog.Warn("failed to get dest stop IDs", "destCode", destCode, "error", err)
		}
	}

	// Pre-fetch bulk data to avoid N+1 Redis calls per candidate.
	unionDeps := rc.GetUnionDepartures(ctx)
	unionByTrip := make(map[string]models.UnionDeparture, len(unionDeps))
	for _, ud := range unionDeps {
		unionByTrip[ud.TripNumber] = ud
	}
	glanceAll := rc.GetAllServiceGlanceMap(ctx)

	result := make([]models.Departure, 0, len(candidates))
	for i := range candidates {
		c := &candidates[i]
		serviceDay, _ := time.ParseInLocation("2006-01-02", c.ServiceDay, torontoLoc)
		scheduled := serviceDay.Add(time.Duration(c.DepartureNano))

		// Apply real-time delay.
		delay := findDelay(ctx, c.TripID, c.StopID, rc)
		adjusted := scheduled.Add(delay)
		delayMin := int(delay.Minutes())

		status := "On Time"
		if update, ok := findTripUpdate(ctx, c.TripID, rc); ok {
			if update.ScheduleRelationship == "CANCELED" {
				status = "Cancelled"
			} else if delayMin >= 1 {
				status = fmt.Sprintf("Delayed +%dm", delayMin)
			}
		}

		dep := models.Departure{
			Line:          c.RouteShortName,
			LineName:      c.RouteLongName,
			Destination:   c.Headsign,
			ScheduledTime: c.ScheduledTime,
			Status:        status,
			Platform:      c.Platform,
			RouteColor:    c.RouteColor,
			DelayMinutes:  delayMin,
			Stops:         c.Stops,
			IsExpress:     c.IsExpress,
			RouteType:     c.RouteType,
			TripNumber:    c.TripNumber,
		}
		if delayMin > 0 {
			dep.ActualTime = formatTime(adjusted)
		}
		if len(destStopIDs) > 0 {
			arr, err := sc.ArrivalTimeAtStop(c.TripID, destStopIDs, []string{c.StopID})
			if err != nil {
				slog.Debug("failed to get arrival time", "tripID", c.TripID, "error", err)
				continue
			}
			if !arr.OK {
				continue
			}
			dep.ArrivalTime = formatTime(serviceDay.Add(time.Duration(arr.Duration)))
		}

		// Enrich with service glance data.
		if sg, ok := glanceAll[c.TripNumber]; ok {
			dep.Cars = sg.Cars
			dep.IsInMotion = sg.IsInMotion
			// Use ServiceGlance delay as fallback when GTFS-RT has no delay.
			if delay == 0 && sg.DelaySeconds > 0 {
				sgDelay := time.Duration(sg.DelaySeconds) * time.Second
				adjusted = scheduled.Add(sgDelay)
				delayMin = int(sgDelay.Minutes())
				dep.DelayMinutes = delayMin
				if delayMin >= 1 {
					dep.Status = fmt.Sprintf("Delayed +%dm", delayMin)
					dep.ActualTime = formatTime(adjusted)
				}
			}
		}

		// Enrich with Union departures board info.
		if ud, ok := unionByTrip[c.TripNumber]; ok {
			isUnion := strings.EqualFold(stopCode, "UN")
			if isUnion && ud.Platform != "" && dep.Platform == "" {
				dep.Platform = ud.Platform
			}
			if ud.Info != "" {
				dep.Status = ud.Info
			}
			if strings.Contains(ud.Info, "CANCEL") {
				dep.IsCancelled = true
				dep.Status = "Cancelled"
			}
		}

		// Flag cancelled trips/stops from exceptions cache.
		if rc.IsStopCancelled(ctx, c.TripNumber, c.StopID) {
			dep.IsCancelled = true
			dep.Status = "Cancelled"
		}

		result = append(result, dep)
	}

	return result
}

// findTripUpdate looks up a trip update by full trip ID first, then by trip number.
func findTripUpdate(ctx context.Context, tripID string, rc *RedisClient) (gtfsrt.RawTripUpdate, bool) {
	if update, ok := rc.GetTripUpdate(ctx, tripID); ok {
		return update, true
	}
	return rc.GetTripUpdate(ctx, models.ExtractTripNumber(tripID))
}

// findDelay returns the departure delay for a trip at a given stop.
func findDelay(ctx context.Context, tripID, stopID string, rc *RedisClient) time.Duration {
	update, ok := findTripUpdate(ctx, tripID, rc)
	if !ok {
		return 0
	}
	var propagated time.Duration
	for _, stu := range update.StopTimeUpdates {
		if stu.DepartureDelay != 0 {
			propagated = stu.DepartureDelay
		}
		if stu.StopID == stopID {
			if stu.DepartureDelay != 0 {
				return stu.DepartureDelay
			}
			return propagated
		}
	}
	return 0
}


// formatTime returns "HH:MM" in local time.
func formatTime(t time.Time) string {
	return fmt.Sprintf("%02d:%02d", t.Hour(), t.Minute())
}
