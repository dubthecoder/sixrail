package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"
	_ "time/tzdata"

	"github.com/teclara/railsix/gtfs-static/store"
	"github.com/teclara/railsix/shared/config"
)

func main() {
	port := config.EnvOr(config.EnvPort, "8081")
	gtfsURL := config.EnvOr(config.EnvGTFSStaticURL, config.DefaultGTFSStaticURL)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	static := store.NewEmptyStaticStore()

	go store.ManageGTFS(ctx, gtfsURL, static, 24*time.Hour)

	mux := http.NewServeMux()
	registerRoutes(mux, static)

	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		slog.Info("starting gtfs-static service", "port", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("server failed", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	slog.Info("shutting down gracefully")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("shutdown error", "error", err)
	}
}

func registerRoutes(mux *http.ServeMux, s *store.StaticStore) {
	mux.HandleFunc("GET /ready", handleReady(s))
	mux.HandleFunc("GET /stops", handleStops(s))
	mux.HandleFunc("GET /stops/{code}/ids", handleStopIDs(s))
	mux.HandleFunc("GET /departures/{stopID}", handleDepartures(s))
	mux.HandleFunc("GET /schedule/{code}", handleSchedule(s))
	mux.HandleFunc("GET /trips/{tripID}", handleTrip(s))
	mux.HandleFunc("GET /routes/{routeID}", handleRoute(s))
	mux.HandleFunc("GET /trips/{tripID}/remaining-stops", handleRemainingStops(s))
	mux.HandleFunc("GET /trips/{tripID}/is-last-stop", handleIsLastStop(s))
	mux.HandleFunc("GET /trips/{tripID}/is-express", handleIsExpress(s))
	mux.HandleFunc("GET /services/{serviceID}/active", handleServiceActive(s))
	mux.HandleFunc("GET /trips/{tripID}/arrival", handleArrival(s))
	mux.HandleFunc("GET /stop-name/{stopID}", handleStopName(s))
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func handleReady(s *store.StaticStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.Ready() {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"status": "loading"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
	}
}

func handleStops(s *store.StaticStore) http.HandlerFunc {
	type slimStop struct {
		ID   string `json:"id"`
		Code string `json:"code"`
		Name string `json:"name"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.Ready() {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "not ready"})
			return
		}
		all := s.AllStops()
		slim := make([]slimStop, len(all))
		for i, st := range all {
			slim[i] = slimStop{ID: st.ID, Code: st.Code, Name: st.Name}
		}
		writeJSON(w, http.StatusOK, slim)
	}
}

func handleStopIDs(s *store.StaticStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.Ready() {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "not ready"})
			return
		}
		code := r.PathValue("code")
		ids := s.StopIDsForCode(code)
		writeJSON(w, http.StatusOK, ids)
	}
}

func handleDepartures(s *store.StaticStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.Ready() {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "not ready"})
			return
		}
		stopID := r.PathValue("stopID")
		deps := s.DeparturesForStop(stopID)
		if deps == nil {
			deps = []store.ScheduledDeparture{}
		}
		writeJSON(w, http.StatusOK, deps)
	}
}

func handleSchedule(s *store.StaticStore) http.HandlerFunc {
	loc, err := time.LoadLocation("America/Toronto")
	if err != nil {
		panic("failed to load America/Toronto timezone: " + err.Error())
	}
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.Ready() {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "not ready"})
			return
		}
		code := r.PathValue("code")
		nowStr := r.URL.Query().Get("now")
		var now time.Time
		if nowStr != "" {
			unix, err := strconv.ParseInt(nowStr, 10, 64)
			if err != nil {
				writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid now param"})
				return
			}
			now = time.Unix(unix, 0).In(loc)
		} else {
			now = time.Now().In(loc)
		}
		candidates := s.ScheduleForStop(code, now, 3*time.Hour)
		if candidates == nil {
			candidates = []store.ScheduleCandidate{}
		}
		writeJSON(w, http.StatusOK, candidates)
	}
}

func handleTrip(s *store.StaticStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.Ready() {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "not ready"})
			return
		}
		tripID := r.PathValue("tripID")
		trip, ok := s.GetTrip(tripID)
		if !ok {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "trip not found"})
			return
		}
		writeJSON(w, http.StatusOK, trip)
	}
}

func handleRoute(s *store.StaticStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.Ready() {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "not ready"})
			return
		}
		routeID := r.PathValue("routeID")
		route, ok := s.GetRoute(routeID)
		if !ok {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "route not found"})
			return
		}
		writeJSON(w, http.StatusOK, route)
	}
}

func handleRemainingStops(s *store.StaticStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.Ready() {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "not ready"})
			return
		}
		tripID := r.PathValue("tripID")
		stopIDs := r.URL.Query()["stopID"]
		names := s.RemainingStopNames(tripID, stopIDs)
		if names == nil {
			names = []string{}
		}
		writeJSON(w, http.StatusOK, names)
	}
}

func handleIsLastStop(s *store.StaticStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.Ready() {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "not ready"})
			return
		}
		tripID := r.PathValue("tripID")
		stopIDs := r.URL.Query()["stopID"]
		result := s.IsLastStop(tripID, stopIDs)
		writeJSON(w, http.StatusOK, map[string]bool{"isLastStop": result})
	}
}

func handleIsExpress(s *store.StaticStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.Ready() {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "not ready"})
			return
		}
		tripID := r.PathValue("tripID")
		result := s.IsExpress(tripID)
		writeJSON(w, http.StatusOK, map[string]bool{"isExpress": result})
	}
}

func handleServiceActive(s *store.StaticStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.Ready() {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "not ready"})
			return
		}
		serviceID := r.PathValue("serviceID")
		dateStr := r.URL.Query().Get("date")
		if dateStr == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "date query param required (YYYY-MM-DD)"})
			return
		}
		date, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid date format, use YYYY-MM-DD"})
			return
		}
		active := s.IsServiceActive(serviceID, date)
		writeJSON(w, http.StatusOK, map[string]bool{"active": active})
	}
}

func handleArrival(s *store.StaticStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.Ready() {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "not ready"})
			return
		}
		tripID := r.PathValue("tripID")
		destIDs := r.URL.Query()["destID"]
		originIDs := r.URL.Query()["originID"]
		dur, ok := s.ArrivalTimeAtStop(tripID, destIDs, originIDs...)
		writeJSON(w, http.StatusOK, store.ArrivalResult{
			Duration: int64(dur),
			OK:       ok,
		})
	}
}

func handleStopName(s *store.StaticStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.Ready() {
			writeJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "not ready"})
			return
		}
		stopID := r.PathValue("stopID")
		name := s.GetStopName(stopID)
		writeJSON(w, http.StatusOK, map[string]string{"name": name})
	}
}
