package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"regexp"
	"time"

	gtfsstore "github.com/teclara/sixrail/api/internal/gtfs"
	"github.com/teclara/sixrail/api/internal/metrolinx"
	"github.com/teclara/sixrail/api/internal/models"
)

var stopCodeRe = regexp.MustCompile(`^[A-Za-z0-9]{2,10}$`)

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

// Alerts serves enriched alerts from the realtime cache.
func (h *Handlers) Alerts(w http.ResponseWriter, r *http.Request) {
	alerts := h.rt.GetAlerts()
	if alerts == nil {
		alerts = []models.Alert{}
	}
	respondJSON(w, alerts)
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
	if h.mx != nil && len(departures) > 0 {
		if nsLines, err := h.mx.GetNextService(r.Context(), stopCode); err == nil {
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

// NetworkHealth returns the count of active trains per GO Transit line.
func (h *Handlers) NetworkHealth(w http.ResponseWriter, r *http.Request) {
	entries := h.rt.GetAllServiceGlance()
	type lineAgg struct {
		name  string
		count int
	}
	byLine := make(map[string]*lineAgg)
	for _, e := range entries {
		if e.LineCode == "" {
			continue
		}
		if agg, ok := byLine[e.LineCode]; ok {
			agg.count++
		} else {
			byLine[e.LineCode] = &lineAgg{name: e.LineName, count: 1}
		}
	}
	result := make([]models.NetworkLine, 0, len(byLine))
	for code, agg := range byLine {
		result = append(result, models.NetworkLine{
			LineCode:    code,
			LineName:    agg.name,
			ActiveTrips: agg.count,
		})
	}
	respondJSON(w, result)
}

// Fares returns fare information between two stations.
func (h *Handlers) Fares(w http.ResponseWriter, r *http.Request) {
	fromCode := r.PathValue("from")
	toCode := r.PathValue("to")
	if !stopCodeRe.MatchString(fromCode) || !stopCodeRe.MatchString(toCode) {
		jsonError(w, "invalid stop code", http.StatusBadRequest)
		return
	}
	if h.mx == nil {
		respondJSON(w, []models.FareInfo{})
		return
	}
	fares, err := h.mx.GetFares(r.Context(), fromCode, toCode)
	if err != nil {
		slog.Warn("fares fetch failed", "error", err)
		respondJSON(w, []models.FareInfo{})
		return
	}
	respondJSON(w, fares)
}

// UnionDepartures returns live departures from Union Station via the Metrolinx REST API.
func (h *Handlers) UnionDepartures(w http.ResponseWriter, r *http.Request) {
	if h.mx == nil {
		respondJSON(w, []models.UnionDeparture{})
		return
	}
	deps, err := h.mx.GetUnionDepartures(r.Context())
	if err != nil {
		slog.Warn("union departures fetch failed", "error", err)
		respondJSON(w, []models.UnionDeparture{})
		return
	}
	respondJSON(w, deps)
}
