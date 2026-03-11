package main

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"regexp"
	"strings"
	"syscall"
	"time"
	_ "time/tzdata"

	"github.com/teclara/railsix/shared/cache"
	"github.com/teclara/railsix/shared/config"
	"github.com/teclara/railsix/shared/metrolinx"
	"github.com/teclara/railsix/shared/models"
)

var stopCodeRe = regexp.MustCompile(`^[A-Za-z0-9]{2,10}$`)

// --- Slim response types ---

type alertResponse struct {
	Headline    string   `json:"headline"`
	Description string   `json:"description"`
	RouteNames  []string `json:"routeNames,omitempty"`
}

type departuresEnvelope struct {
	StationAlert string              `json:"stationAlert,omitempty"`
	Departures   []departureResponse `json:"departures"`
}

type departureResponse struct {
	Line          string   `json:"line"`
	LineName      string   `json:"lineName,omitempty"`
	ScheduledTime string   `json:"scheduledTime"`
	ActualTime    string   `json:"actualTime,omitempty"`
	ArrivalTime   string   `json:"arrivalTime,omitempty"`
	Status        string   `json:"status"`
	Platform      string   `json:"platform,omitempty"`
	DelayMinutes  int      `json:"delayMinutes,omitempty"`
	Stops         []string `json:"stops,omitempty"`
	Cars          string   `json:"cars,omitempty"`
	IsInMotion    bool     `json:"isInMotion,omitempty"`
	IsCancelled   bool     `json:"isCancelled,omitempty"`
	IsExpress     bool     `json:"isExpress,omitempty"`
	Alert         string   `json:"alert,omitempty"`
	RouteType     int      `json:"routeType"`
}

type unionDepartureResponse struct {
	Service     string   `json:"service"`
	ServiceType string   `json:"serviceType,omitempty"`
	Platform    string   `json:"platform"`
	Time        string   `json:"time"`
	Info        string   `json:"info"`
	Stops       []string `json:"stops"`
	Cars        string   `json:"cars,omitempty"`
	IsInMotion  bool     `json:"isInMotion,omitempty"`
	IsCancelled bool     `json:"isCancelled,omitempty"`
	Alert       string   `json:"alert,omitempty"`
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

func main() {
	port := config.EnvOr(config.EnvPort, "8082")
	redisAddr := config.EnvOr(config.EnvRedisAddr, config.DefaultRedisAddr)
	redisPassword := config.EnvOr(config.EnvRedisPassword, "")
	gtfsStaticAddr := config.EnvOr(config.EnvGTFSStaticAddr, config.DefaultGTFSStaticAddr)
	mxBase := config.EnvOr(config.EnvMetrolinxBase, config.DefaultMetrolinxBase)
	mxKey := os.Getenv(config.EnvMetrolinxAPIKey)

	rc, err := cache.Connect(redisAddr, redisPassword)
	if err != nil {
		slog.Error("failed to connect to Redis", "error", err)
		os.Exit(1)
	}
	defer rc.Close()
	slog.Info("connected to Redis", "addr", redisAddr)

	redisClient := NewRedisClient(rc)
	staticClient := NewStaticClient(gtfsStaticAddr)

	var mx *metrolinx.Client
	if mxKey != "" {
		mx = metrolinx.NewClient(mxBase, mxKey)
		slog.Info("Metrolinx client configured")
	} else {
		slog.Warn("METROLINX_API_KEY not set, NextService and Fares will be unavailable")
	}

	mux := http.NewServeMux()
	registerRoutes(mux, staticClient, redisClient, mx)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		slog.Info("starting departures-api service", "port", port)
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

func registerRoutes(mux *http.ServeMux, sc *StaticClient, rc *RedisClient, mx *metrolinx.Client) {
	mux.HandleFunc("GET /health", handleHealth(sc, rc))
	mux.HandleFunc("GET /stops", handleStops(sc))
	mux.HandleFunc("GET /departures/{stopCode}", handleDepartures(sc, rc, mx))
	mux.HandleFunc("GET /union-departures", handleUnionDepartures(rc))
	mux.HandleFunc("GET /network-health", handleNetworkHealth(rc))
	mux.HandleFunc("GET /alerts", handleAlerts(rc))
}

func handleStops(sc *StaticClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data, err := sc.GetStops()
		if err != nil {
			slog.Warn("stops proxy failed", "error", err)
			jsonError(w, "unable to fetch stops", http.StatusBadGateway)
			return
		}
		slog.Info("stops proxy ok", "bytes", len(data))
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "public, max-age=3600")
		w.WriteHeader(http.StatusOK)
		if _, writeErr := w.Write(data); writeErr != nil {
			slog.Warn("write stops response failed", "error", writeErr)
		}
	}
}

func handleDepartures(sc *StaticClient, rc *RedisClient, mx *metrolinx.Client) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		stopCode := r.PathValue("stopCode")
		if !stopCodeRe.MatchString(stopCode) {
			jsonError(w, "invalid stop code", http.StatusBadRequest)
			return
		}
		destCode := r.URL.Query().Get("dest")
		if destCode != "" && !stopCodeRe.MatchString(destCode) {
			destCode = ""
		}

		departures := GetDepartures(r.Context(), stopCode, destCode, time.Now(), sc, rc)

		// Enrich with NextService real-time data (cached with 30s TTL).
		if mx != nil && len(departures) > 0 {
			nsLines, ok := rc.GetNextService(r.Context(), stopCode)
			if !ok {
				nsCtx, nsCancel := context.WithTimeout(r.Context(), 3*time.Second)
				if fetched, err := mx.GetNextService(nsCtx, stopCode); err == nil {
					nsLines = fetched
					rc.SetNextService(r.Context(), stopCode, fetched)
				} else {
					slog.Warn("NextService fetch failed", "stopCode", stopCode, "error", err)
				}
				nsCancel()
			}
			if nsLines != nil {
				byTrip := make(map[string]*models.NextServiceLine, len(nsLines))
				for i := range nsLines {
					byTrip[nsLines[i].TripNumber] = &nsLines[i]
				}
				for i := range departures {
					ns := byTrip[departures[i].TripNumber]
					if ns == nil {
						continue
					}
					if ns.ActualPlatform != "" {
						departures[i].Platform = ns.ActualPlatform
					} else if ns.Platform != "" && departures[i].Platform == "" {
						departures[i].Platform = ns.Platform
					}
				}
			}
		}

		// Enrich with cached Union departures for platform data.
		if unionDeps := rc.GetUnionDepartures(r.Context()); len(unionDeps) > 0 {
			type udKey struct{ service, time string }
			udMap := make(map[udKey]string, len(unionDeps))
			for _, ud := range unionDeps {
				p := strings.TrimSpace(ud.Platform)
				if p != "" && p != "-" {
					udMap[udKey{strings.ToUpper(ud.Service), ud.Time}] = p
				}
			}
			for i := range departures {
				if departures[i].Platform != "" {
					continue
				}
				key := udKey{strings.ToUpper(departures[i].LineName), departures[i].ScheduledTime}
				if p, ok := udMap[key]; ok {
					departures[i].Platform = p
				}
			}
		}

		// Return slim response with station alert envelope.
		alertTexts := routeAlertTexts(rc, r.Context())
		stopAlerts := stopAlertTexts(rc, r.Context())

		// Check if the queried station has a stop-specific alert.
		var stationAlert string
		if len(stopAlerts) > 0 {
			stationStopIDs, err := sc.StopIDsForCode(stopCode)
			if err == nil {
				for _, sid := range stationStopIDs {
					if headline, ok := stopAlerts[sid]; ok {
						stationAlert = headline
						break
					}
				}
			}
		}

		slim := make([]departureResponse, len(departures))
		for i, d := range departures {
			slim[i] = departureResponse{
				Line:          d.Line,
				LineName:      d.LineName,
				ScheduledTime: d.ScheduledTime,
				ActualTime:    d.ActualTime,
				ArrivalTime:   d.ArrivalTime,
				Status:        d.Status,
				Platform:      d.Platform,
				DelayMinutes:  d.DelayMinutes,
				Stops:         d.Stops,
				Cars:          d.Cars,
				IsInMotion:    d.IsInMotion,
				IsCancelled:   d.IsCancelled,
				IsExpress:     d.IsExpress,
				Alert:         alertTexts[strings.ToUpper(d.Line)],
				RouteType:     d.RouteType,
			}
		}
		respondJSON(w, departuresEnvelope{
			StationAlert: stationAlert,
			Departures:   slim,
		})
	}
}

func handleUnionDepartures(rc *RedisClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		deps := rc.GetUnionDepartures(r.Context())
		if deps == nil {
			respondJSON(w, []unionDepartureResponse{})
			return
		}
		alertTexts := routeAlertTexts(rc, r.Context())
		slim := make([]unionDepartureResponse, len(deps))
		for i, d := range deps {
			slim[i] = unionDepartureResponse{
				Service:     d.Service,
				ServiceType: d.ServiceType,
				Platform:    d.Platform,
				Time:        d.Time,
				Info:        d.Info,
				Stops:       d.Stops,
				IsCancelled: strings.Contains(strings.ToUpper(d.Info), "CANCEL"),
				Alert:       alertTexts[lineCodeForService[strings.ToUpper(d.Service)]],
			}
			if sg, ok := rc.GetServiceGlanceEntry(r.Context(), d.TripNumber); ok {
				slim[i].Cars = sg.Cars
				slim[i].IsInMotion = sg.IsInMotion
			}
			if rc.IsTripCancelled(r.Context(), d.TripNumber) {
				slim[i].IsCancelled = true
			}
			if update, ok := rc.GetTripUpdate(r.Context(), d.TripNumber); ok {
				if update.ScheduleRelationship == "CANCELED" {
					slim[i].IsCancelled = true
				}
			}
		}
		respondJSON(w, slim)
	}
}

func handleNetworkHealth(rc *RedisClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		entries := rc.GetAllServiceGlance(r.Context())
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
		w.Header().Set("Cache-Control", "public, max-age=30")
		respondJSON(w, result)
	}
}

func handleAlerts(rc *RedisClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		alerts := rc.GetAlerts(r.Context())
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
		w.Header().Set("Cache-Control", "public, max-age=30")
		respondJSON(w, slim)
	}
}

// stopAlertTexts returns a map of stop ID -> alert headline for stop-specific alerts.
func stopAlertTexts(rc *RedisClient, ctx context.Context) map[string]string {
	alerts := rc.GetAlerts(ctx)
	m := make(map[string]string)
	for _, a := range alerts {
		for _, sid := range a.StopIDs {
			if _, exists := m[sid]; !exists {
				m[sid] = a.Headline
			}
		}
	}
	return m
}

// routeAlertTexts returns a map of uppercased line code -> alert headline.
// Line codes are extracted from the route_id suffix (e.g. "01260426-ST" → "ST").
// This avoids collisions between bus and train routes that share a display name
// (e.g. bus route 71 "Stouffville" vs train route ST "Stouffville").
func routeAlertTexts(rc *RedisClient, ctx context.Context) map[string]string {
	alerts := rc.GetAlerts(ctx)
	m := make(map[string]string)
	for _, a := range alerts {
		for _, rid := range a.RouteIDs {
			code := rid
			if idx := strings.LastIndex(rid, "-"); idx >= 0 && idx+1 < len(rid) {
				code = rid[idx+1:]
			}
			key := strings.ToUpper(code)
			if _, exists := m[key]; !exists {
				m[key] = a.Headline
			}
		}
	}
	return m
}

// lineCodeForService maps uppercased service display names to GO Train line codes.
var lineCodeForService = func() map[string]string {
	m := make(map[string]string, len(allLines))
	for _, l := range allLines {
		m[strings.ToUpper(l.name)] = strings.ToUpper(l.code)
	}
	return m
}()

// --- JSON helpers ---

func respondJSON(w http.ResponseWriter, v any) {
	respondJSONStatus(w, http.StatusOK, v)
}

func respondJSONStatus(w http.ResponseWriter, status int, v any) {
	data, err := json.Marshal(v)
	if err != nil {
		jsonError(w, "internal error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if _, writeErr := w.Write(data); writeErr != nil {
		slog.Warn("write response failed", "error", writeErr)
	}
}

func jsonError(w http.ResponseWriter, msg string, status int) {
	resp := struct {
		Error string `json:"error"`
	}{Error: msg}
	data, _ := json.Marshal(resp)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if _, err := w.Write(data); err != nil {
		slog.Warn("write error response failed", "error", err)
	}
}
