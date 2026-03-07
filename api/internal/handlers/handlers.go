package handlers

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"regexp"
	"time"

	gtfsstore "github.com/teclara/sixrail/api/internal/gtfs"
	"github.com/teclara/sixrail/api/internal/metrolinx"
	"github.com/teclara/sixrail/api/internal/models"
)

var stopCodeRe = regexp.MustCompile(`^[A-Za-z0-9]{2,10}$`)
var tripIDRe = regexp.MustCompile(`^[A-Za-z0-9._-]{1,80}$`)

type Handlers struct {
	static *gtfsstore.StaticStore
	rt     *gtfsstore.RealtimeCache
	mx     *metrolinx.Client // nil when no API key is configured
}

func New(static *gtfsstore.StaticStore, rt *gtfsstore.RealtimeCache, mx *metrolinx.Client) *Handlers {
	return &Handlers{static: static, rt: rt, mx: mx}
}

func (h *Handlers) Health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"ok"}`))
}

func writeJSON(w http.ResponseWriter, status int, data []byte) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if _, err := w.Write(data); err != nil {
		slog.Warn("write response failed", "error", err)
	}
}

func jsonError(w http.ResponseWriter, msg string, status int) {
	writeJSON(w, status, []byte(`{"error":"`+msg+`"}`))
}

func respondJSON(w http.ResponseWriter, v any) {
	data, err := json.Marshal(v)
	if err != nil {
		jsonError(w, "internal error", http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, data)
}

// AllStops serves stops from GTFS static data.
func (h *Handlers) AllStops(w http.ResponseWriter, r *http.Request) {
	stops := h.static.AllStops()
	respondJSON(w, stops)
}

// Positions serves enriched vehicle positions from the realtime cache.
func (h *Handlers) Positions(w http.ResponseWriter, r *http.Request) {
	positions := h.rt.GetPositions()
	if positions == nil {
		positions = []models.VehiclePosition{}
	}
	respondJSON(w, positions)
}

// Alerts serves enriched alerts from the realtime cache.
func (h *Handlers) Alerts(w http.ResponseWriter, r *http.Request) {
	alerts := h.rt.GetAlerts()
	if alerts == nil {
		alerts = []models.Alert{}
	}
	respondJSON(w, alerts)
}

// RouteShapes serves rail route shapes from GTFS static data.
func (h *Handlers) RouteShapes(w http.ResponseWriter, r *http.Request) {
	shapes := h.static.RouteShapes()
	if shapes == nil {
		shapes = []models.RouteShape{}
	}
	respondJSON(w, shapes)
}

// TripDetail returns enriched trip info for a vehicle hover popup.
func (h *Handlers) TripDetail(w http.ResponseWriter, r *http.Request) {
	tripID := r.PathValue("tripId")
	if !tripIDRe.MatchString(tripID) {
		jsonError(w, "invalid trip ID", http.StatusBadRequest)
		return
	}

	// Find the vehicle position for this trip
	var vp *models.VehiclePosition
	for _, p := range h.rt.GetPositions() {
		if p.TripID == tripID {
			vp = &p
			break
		}
	}
	if vp == nil {
		jsonError(w, "trip not found", http.StatusNotFound)
		return
	}

	// Get the static trip schedule
	simTrip, hasSchedule := h.static.GetSimTrip(tripID)

	// Get the real-time trip update for delays
	tripUpdate, hasUpdate := h.rt.GetTripUpdate(tripID)

	// Build delay map: stopID → delay seconds
	delayMap := make(map[string]int)
	maxDelay := 0
	if hasUpdate {
		for _, stu := range tripUpdate.StopTimeUpdates {
			delay := int(stu.DepartureDelay.Seconds())
			if delay == 0 {
				delay = int(stu.ArrivalDelay.Seconds())
			}
			delayMap[stu.StopID] = delay
			if delay > maxDelay {
				maxDelay = delay
			}
		}
	}

	delayMinutes := maxDelay / 60
	status := "On Time"
	if hasUpdate && tripUpdate.ScheduleRelationship == "CANCELED" {
		status = "Cancelled"
	} else if delayMinutes > 0 {
		status = fmt.Sprintf("Delayed +%dm", delayMinutes)
	}

	loc, _ := time.LoadLocation("America/Toronto")
	now := time.Now().In(loc)

	detail := models.TripDetail{
		TripID:       vp.TripID,
		VehicleID:    vp.VehicleID,
		RouteName:    vp.RouteName,
		RouteColor:   vp.RouteColor,
		Status:       status,
		DelayMinutes: delayMinutes,
		CurrentStop:  h.static.GetStopName(vp.NextStopID),
	}

	if hasSchedule && len(simTrip.Stops) >= 2 {
		detail.Origin = h.static.GetStopName(simTrip.Stops[0].StopID)
		detail.Destination = h.static.GetStopName(simTrip.Stops[len(simTrip.Stops)-1].StopID)

		midnight := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
		startTime := midnight.Add(simTrip.Stops[0].DepartureTime)
		endTime := midnight.Add(simTrip.Stops[len(simTrip.Stops)-1].ArrivalTime)
		detail.ScheduleStart = startTime.Format("15:04")
		detail.ScheduleEnd = endTime.Format("15:04")

		// Build upcoming stops (stops after the current time)
		upcoming := make([]models.UpcomingStop, 0)
		nowOffset := now.Sub(midnight)
		for _, ts := range simTrip.Stops {
			if ts.ArrivalTime <= nowOffset {
				continue
			}
			stopDelay := delayMap[ts.StopID]
			arrTime := midnight.Add(ts.ArrivalTime).Add(time.Duration(stopDelay) * time.Second)
			upcoming = append(upcoming, models.UpcomingStop{
				Name:         h.static.GetStopName(ts.StopID),
				Time:         arrTime.Format("3:04 p.m."),
				DelayMinutes: stopDelay / 60,
			})
		}
		detail.UpcomingStops = upcoming
	}

	if detail.UpcomingStops == nil {
		detail.UpcomingStops = []models.UpcomingStop{}
	}

	respondJSON(w, detail)
}

// StopDepartures returns GTFS-based departures for a stop code, enriched with
// real-time NextService data (platform + computed time) when available.
// Optional query param ?dest=<stopCode> populates ArrivalTime at the destination.
func (h *Handlers) StopDepartures(w http.ResponseWriter, r *http.Request) {
	stopCode := r.PathValue("stopCode")
	if !stopCodeRe.MatchString(stopCode) {
		jsonError(w, "invalid stop code", http.StatusBadRequest)
		return
	}
	destCode := r.URL.Query().Get("dest")
	if destCode != "" && !stopCodeRe.MatchString(destCode) {
		destCode = ""
	}
	departures := gtfsstore.GetDepartures(stopCode, destCode, time.Now(), h.static, h.rt)

	// Enrich with Metrolinx NextService real-time data when available.
	// NextService returns one entry per line/direction (the very next departure).
	// We match by line code and pick the NS entry whose ComputedTime is within
	// 10 minutes of the GTFS scheduled time.
	if h.mx != nil && len(departures) > 0 {
		if nsLines, err := h.mx.GetNextService(r.Context(), stopCode); err == nil {
			// Build a map: lineCode → []NextServiceLine
			byLine := make(map[string][]models.NextServiceLine, len(nsLines))
			for _, l := range nsLines {
				byLine[l.LineCode] = append(byLine[l.LineCode], l)
			}
			for i := range departures {
				candidates := byLine[departures[i].Line]
				ns := bestNSMatch(departures[i].ScheduledTime, candidates)
				if ns == nil {
					continue
				}
				if ns.ComputedTime != "--:--" {
					departures[i].ScheduledTime = ns.ComputedTime
				}
				if ns.ActualPlatform != "" {
					departures[i].Platform = ns.ActualPlatform
				} else if ns.Platform != "" && departures[i].Platform == "" {
					departures[i].Platform = ns.Platform
				}
			}
		}
	}

	respondJSON(w, departures)
}

// bestNSMatch returns the NextServiceLine whose ComputedTime is within 10 minutes
// of the given "HH:MM" scheduled time, or nil if none match.
func bestNSMatch(scheduledHHMM string, candidates []models.NextServiceLine) *models.NextServiceLine {
	sched, err := time.Parse("15:04", scheduledHHMM)
	if err != nil {
		return nil
	}
	const window = 10 * time.Minute
	for i := range candidates {
		comp, err := time.Parse("15:04", candidates[i].ComputedTime)
		if err != nil {
			continue
		}
		diff := comp.Sub(sched)
		if diff < 0 {
			diff = -diff
		}
		if diff <= window {
			return &candidates[i]
		}
	}
	return nil
}

// UnionDepartures returns live departures from Union Station via the Metrolinx REST API.
func (h *Handlers) UnionDepartures(w http.ResponseWriter, r *http.Request) {
	if h.mx == nil {
		respondJSON(w, []models.UnionDeparture{})
		return
	}
	mode := r.URL.Query().Get("mode")
	deps, err := h.mx.GetUnionDepartures(r.Context(), mode)
	if err != nil {
		slog.Warn("union departures fetch failed", "error", err)
		respondJSON(w, []models.UnionDeparture{})
		return
	}
	respondJSON(w, deps)
}
