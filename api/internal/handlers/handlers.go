package handlers

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"regexp"
	"time"

	"github.com/teclara/gopulse/api/internal/cache"
	gtfsstore "github.com/teclara/gopulse/api/internal/gtfs"
	"github.com/teclara/gopulse/api/internal/models"
)

var stopCodeRe = regexp.MustCompile(`^[A-Za-z0-9]{2,10}$`)

type Fetcher interface {
	Fetch(ctx context.Context, path string) ([]byte, error)
}

type Handlers struct {
	fetcher Fetcher
	cache   *cache.Cache
	static  *gtfsstore.StaticStore
	rt      *gtfsstore.RealtimeCache
}

func New(fetcher Fetcher, cache *cache.Cache, static *gtfsstore.StaticStore, rt *gtfsstore.RealtimeCache) *Handlers {
	return &Handlers{fetcher: fetcher, cache: cache, static: static, rt: rt}
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

// StopDepartures still proxies via the Metrolinx REST API (no GTFS-RT equivalent).
func (h *Handlers) StopDepartures(w http.ResponseWriter, r *http.Request) {
	stopCode := r.PathValue("stopCode")
	if !stopCodeRe.MatchString(stopCode) {
		jsonError(w, "invalid stop code", http.StatusBadRequest)
		return
	}
	h.cachedProxy(w, r, "/Stop/NextService/"+stopCode, 30*time.Second)
}

func (h *Handlers) cachedProxy(w http.ResponseWriter, r *http.Request, metrolinxPath string, ttl time.Duration) {
	if data, ok := h.cache.Get(metrolinxPath); ok {
		w.Header().Set("X-Cache", "HIT")
		writeJSON(w, http.StatusOK, data)
		return
	}

	data, err := h.fetcher.Fetch(r.Context(), metrolinxPath)
	if err != nil {
		slog.Error("metrolinx fetch failed", "path", metrolinxPath, "error", err)
		if stale, ok := h.cache.GetStale(metrolinxPath); ok {
			w.Header().Set("X-Cache", "STALE")
			w.Header().Set("X-Cache-Stale", "true")
			writeJSON(w, http.StatusOK, stale)
			return
		}
		jsonError(w, "upstream unavailable", http.StatusBadGateway)
		return
	}

	h.cache.Set(metrolinxPath, data, ttl)
	w.Header().Set("X-Cache", "MISS")
	writeJSON(w, http.StatusOK, data)
}
