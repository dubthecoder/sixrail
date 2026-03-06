package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"regexp"
	"time"

	gtfsstore "github.com/teclara/sixrail/api/internal/gtfs"
	"github.com/teclara/sixrail/api/internal/models"
)

var stopCodeRe = regexp.MustCompile(`^[A-Za-z0-9]{2,10}$`)

type Handlers struct {
	static *gtfsstore.StaticStore
	rt     *gtfsstore.RealtimeCache
}

func New(static *gtfsstore.StaticStore, rt *gtfsstore.RealtimeCache) *Handlers {
	return &Handlers{static: static, rt: rt}
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

// StopDepartures returns GTFS-based departures for a stop code.
func (h *Handlers) StopDepartures(w http.ResponseWriter, r *http.Request) {
	stopCode := r.PathValue("stopCode")
	if !stopCodeRe.MatchString(stopCode) {
		jsonError(w, "invalid stop code", http.StatusBadRequest)
		return
	}
	departures := gtfsstore.GetDepartures(stopCode, time.Now(), h.static, h.rt)
	respondJSON(w, departures)
}
