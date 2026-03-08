package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
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

// --- Slim response types (only fields the frontend uses) ---

type stopResponse struct {
	ID   string `json:"id"`
	Code string `json:"code"`
	Name string `json:"name"`
}

type alertResponse struct {
	Headline    string   `json:"headline"`
	Description string   `json:"description"`
	RouteNames  []string `json:"routeNames,omitempty"`
}

type departureResponse struct {
	Line          string   `json:"line"`
	LineName      string   `json:"lineName,omitempty"`
	ScheduledTime string   `json:"scheduledTime"`
	ArrivalTime   string   `json:"arrivalTime,omitempty"`
	Status        string   `json:"status"`
	Platform      string   `json:"platform,omitempty"`
	DelayMinutes  int      `json:"delayMinutes,omitempty"`
	Stops         []string `json:"stops,omitempty"`
	Occupancy     string   `json:"occupancy,omitempty"`
	Cars          string   `json:"cars,omitempty"`
	IsInMotion    bool     `json:"isInMotion,omitempty"`
	IsCancelled   bool     `json:"isCancelled,omitempty"`
	Alert         string   `json:"alert,omitempty"`
	RouteType     int      `json:"routeType"`
}

type unionDepartureResponse struct {
	Service     string   `json:"service"`
	Platform    string   `json:"platform"`
	Time        string   `json:"time"`
	Info        string   `json:"info"`
	Stops       []string `json:"stops"`
	Cars        string   `json:"cars,omitempty"`
	Occupancy   string   `json:"occupancy,omitempty"`
	IsInMotion  bool     `json:"isInMotion,omitempty"`
	IsCancelled bool     `json:"isCancelled,omitempty"`
	Alert       string   `json:"alert,omitempty"`
}

type fareResponse struct {
	Category string  `json:"category"`
	FareType string  `json:"fareType"`
	Amount   float64 `json:"amount"`
}

// routeAlertTexts returns a map of uppercased route name → alert headline.
func (h *Handlers) routeAlertTexts() map[string]string {
	alerts := h.rt.GetAlerts()
	m := make(map[string]string)
	for _, a := range alerts {
		for _, name := range a.RouteNames {
			key := strings.ToUpper(name)
			if _, exists := m[key]; !exists {
				m[key] = a.Headline
			}
		}
	}
	return m
}

// AllStops serves stops from GTFS static data (slim: no lat/lon/parentId).
func (h *Handlers) AllStops(w http.ResponseWriter, r *http.Request) {
	stops := h.static.AllStops()
	slim := make([]stopResponse, len(stops))
	for i, s := range stops {
		slim[i] = stopResponse{ID: s.ID, Code: s.Code, Name: s.Name}
	}
	respondJSON(w, slim)
}

// Alerts serves enriched alerts from the realtime cache (slim: headline, description, routeNames only).
func (h *Handlers) Alerts(w http.ResponseWriter, r *http.Request) {
	alerts := h.rt.GetAlerts()
	if alerts == nil {
		respondJSON(w, []alertResponse{})
		return
	}
	slim := make([]alertResponse, len(alerts))
	for i, a := range alerts {
		slim[i] = alertResponse{
			Headline:    a.Headline,
			Description: a.Description,
			RouteNames:  a.RouteNames,
		}
	}
	respondJSON(w, slim)
}

// StopDepartures returns GTFS-based departures for a stop code, enriched with
// real-time NextService data (platform + computed time) when available.
// Uses a 30s TTL cache for NextService to avoid per-request upstream calls.
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

	// Enrich with NextService real-time data (cached with 30s TTL).
	if h.mx != nil && len(departures) > 0 {
		nsLines, ok := h.rt.GetNextService(stopCode)
		if !ok {
			if fetched, err := h.mx.GetNextService(r.Context(), stopCode); err == nil {
				nsLines = fetched
				h.rt.SetNextService(stopCode, fetched)
			}
		}
		if nsLines != nil {
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

	// Return slim response (no destination, routeColor).
	alertTexts := h.routeAlertTexts()
	slim := make([]departureResponse, len(departures))
	for i, d := range departures {
		slim[i] = departureResponse{
			Line:          d.Line,
			LineName:      d.LineName,
			ScheduledTime: d.ScheduledTime,
			ArrivalTime:   d.ArrivalTime,
			Status:        d.Status,
			Platform:      d.Platform,
			DelayMinutes:  d.DelayMinutes,
			Stops:         d.Stops,
			Occupancy:     d.Occupancy,
			Cars:          d.Cars,
			IsInMotion:    d.IsInMotion,
			IsCancelled:   d.IsCancelled,
			Alert:         alertTexts[strings.ToUpper(d.LineName)],
			RouteType:     d.RouteType,
		}
	}
	respondJSON(w, slim)
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

// All GO Transit train lines with their codes and display names.
var allLines = []struct {
	code string
	name string
}{
	{"BR", "Barrie"},
	{"GT", "Georgetown"},
	{"KI", "Kitchener"},
	{"LE", "Lakeshore East"},
	{"LW", "Lakeshore West"},
	{"MI", "Milton"},
	{"ST", "Stouffville"},
}

// NetworkHealth returns the count of active trains per GO Transit line.
// Always returns all lines, showing 0 for lines with no active trains.
func (h *Handlers) NetworkHealth(w http.ResponseWriter, r *http.Request) {
	entries := h.rt.GetAllServiceGlance()
	counts := make(map[string]int, len(allLines))
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
	respondJSON(w, result)
}

// Fares returns fare information between two stations (cached with 1h TTL).
func (h *Handlers) Fares(w http.ResponseWriter, r *http.Request) {
	fromCode := r.PathValue("from")
	toCode := r.PathValue("to")
	if !stopCodeRe.MatchString(fromCode) || !stopCodeRe.MatchString(toCode) {
		jsonError(w, "invalid stop code", http.StatusBadRequest)
		return
	}
	if h.mx == nil {
		respondJSON(w, []fareResponse{})
		return
	}

	var fares []models.FareInfo
	if cached, ok := h.rt.GetFares(fromCode, toCode); ok {
		fares = cached
	} else {
		fetched, err := h.mx.GetFares(r.Context(), fromCode, toCode)
		if err != nil {
			slog.Warn("fares fetch failed", "error", err)
			respondJSON(w, []fareResponse{})
			return
		}
		fares = fetched
		h.rt.SetFares(fromCode, toCode, fetched)
	}

	slim := make([]fareResponse, len(fares))
	for i, f := range fares {
		slim[i] = fareResponse{
			Category: f.Category,
			FareType: f.FareType,
			Amount:   f.Amount,
		}
	}
	respondJSON(w, slim)
}

// UnionDepartures serves cached Union Station departures (polled every 30s).
func (h *Handlers) UnionDepartures(w http.ResponseWriter, r *http.Request) {
	deps := h.rt.GetUnionDepartures()
	if deps == nil {
		respondJSON(w, []unionDepartureResponse{})
		return
	}
	alertTexts := h.routeAlertTexts()
	slim := make([]unionDepartureResponse, len(deps))
	for i, d := range deps {
		slim[i] = unionDepartureResponse{
			Service:     d.Service,
			Platform:    d.Platform,
			Time:        d.Time,
			Info:        d.Info,
			Stops:       d.Stops,
			IsCancelled: strings.Contains(strings.ToUpper(d.Info), "CANCEL"),
			Alert:       alertTexts[strings.ToUpper(d.Service)],
		}
		if sg, ok := h.rt.GetServiceGlanceEntry(d.TripNumber); ok {
			slim[i].Cars = sg.Cars
			slim[i].IsInMotion = sg.IsInMotion
		}
		if occ := h.rt.GetOccupancyByTripNumber(d.TripNumber); occ != "" {
			slim[i].Occupancy = occ
		}
		if h.rt.IsTripCancelled(d.TripNumber) {
			slim[i].IsCancelled = true
		}
	}
	respondJSON(w, slim)
}
