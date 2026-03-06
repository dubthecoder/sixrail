package gtfs

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/teclara/sixrail/api/internal/models"
)

// RouteLookup is satisfied by StaticStore.
type RouteLookup interface {
	GetRoute(id string) (models.Route, bool)
}

// Fetcher retrieves raw bytes from a remote path.
type Fetcher interface {
	Fetch(ctx context.Context, path string) ([]byte, error)
}

// --- JSON structures matching Metrolinx GTFS-RT JSON format ---

type gtfsRTFeed struct {
	Entity []gtfsRTEntity `json:"entity"`
}

type gtfsRTEntity struct {
	ID         string          `json:"id"`
	Vehicle    *gtfsRTVehicle  `json:"vehicle"`
	Alert      *gtfsRTAlert    `json:"alert"`
	TripUpdate *gtfsRTTripUpd  `json:"trip_update"`
}

type gtfsRTVehicle struct {
	Trip          gtfsRTTrip     `json:"trip"`
	Vehicle       gtfsRTVehID    `json:"vehicle"`
	Position      *gtfsRTPos     `json:"position"`
	StopID        string         `json:"stop_id"`
	CurrentStatus string         `json:"current_status"`
	Timestamp     int64          `json:"timestamp"`
}

type gtfsRTTrip struct {
	TripID               string `json:"trip_id"`
	RouteID              string `json:"route_id"`
	ScheduleRelationship string `json:"schedule_relationship"`
}

type gtfsRTVehID struct {
	ID    string `json:"id"`
	Label string `json:"label"`
}

type gtfsRTPos struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Bearing   float32 `json:"bearing"`
	Speed     float32 `json:"speed"`
}

type gtfsRTAlert struct {
	ActivePeriod   []gtfsRTActivePeriod  `json:"active_period"`
	InformedEntity []gtfsRTInformedEnt   `json:"informed_entity"`
	Effect         string                `json:"effect"`
	URL            *gtfsRTTranslatedStr  `json:"url"`
	HeaderText     *gtfsRTTranslatedStr  `json:"header_text"`
	DescriptionTxt *gtfsRTTranslatedStr  `json:"description_text"`
}

type gtfsRTActivePeriod struct {
	Start int64 `json:"start"`
	End   int64 `json:"end"`
}

type gtfsRTInformedEnt struct {
	RouteID string `json:"route_id"`
}

type gtfsRTTranslatedStr struct {
	Translation []gtfsRTTranslation `json:"translation"`
}

type gtfsRTTranslation struct {
	Text     string `json:"text"`
	Language string `json:"language"`
}

type gtfsRTTripUpd struct {
	Trip           gtfsRTTrip       `json:"trip"`
	StopTimeUpdate []gtfsRTStopTime `json:"stop_time_update"`
}

type gtfsRTStopTime struct {
	StopID               string          `json:"stop_id"`
	Arrival              *gtfsRTDelay    `json:"arrival"`
	Departure            *gtfsRTDelay    `json:"departure"`
	ScheduleRelationship string          `json:"schedule_relationship"`
}

type gtfsRTDelay struct {
	Delay int `json:"delay"`
}

// --- Raw intermediate types ---

// RawPosition holds pre-enrichment vehicle position data.
type RawPosition struct {
	VehicleID     string
	TripID        string
	RouteID       string
	Lat           float64
	Lon           float64
	Bearing       float32
	Speed         float32
	Timestamp     int64
	CurrentStatus string
	NextStopID    string
}

// RawAlert holds pre-enrichment alert data.
type RawAlert struct {
	ID          string
	Effect      string
	Headline    string
	Description string
	URL         string
	RouteIDs    []string
	StartTime   int64
	EndTime     int64
}

// RawStopTimeUpdate holds real-time delay info for one stop within a trip.
type RawStopTimeUpdate struct {
	StopID               string
	ArrivalDelay         time.Duration
	DepartureDelay       time.Duration
	ScheduleRelationship string
}

// RawTripUpdate holds real-time updates for a trip.
type RawTripUpdate struct {
	TripID               string
	RouteID              string
	ScheduleRelationship string
	StopTimeUpdates      []RawStopTimeUpdate
}

// --- Cache ---

// RealtimeCache is a thread-safe store for enriched positions, alerts, and trip updates.
type RealtimeCache struct {
	mu          sync.RWMutex
	positions   []models.VehiclePosition
	alerts      []models.Alert
	tripUpdates map[string]RawTripUpdate
}

func NewRealtimeCache() *RealtimeCache {
	return &RealtimeCache{tripUpdates: make(map[string]RawTripUpdate)}
}

func (rc *RealtimeCache) SetPositions(positions []models.VehiclePosition) {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	rc.positions = positions
}

func (rc *RealtimeCache) GetPositions() []models.VehiclePosition {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	out := make([]models.VehiclePosition, len(rc.positions))
	copy(out, rc.positions)
	return out
}

func (rc *RealtimeCache) SetAlerts(alerts []models.Alert) {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	rc.alerts = alerts
}

func (rc *RealtimeCache) GetAlerts() []models.Alert {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	out := make([]models.Alert, len(rc.alerts))
	copy(out, rc.alerts)
	return out
}

func (rc *RealtimeCache) SetTripUpdates(updates map[string]RawTripUpdate) {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	rc.tripUpdates = updates
}

func (rc *RealtimeCache) GetTripUpdate(tripID string) (RawTripUpdate, bool) {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	u, ok := rc.tripUpdates[tripID]
	return u, ok
}

// --- JSON parsers ---

// ParsePositions parses the Metrolinx GTFS-RT JSON vehicle positions feed.
func ParsePositions(data []byte) ([]RawPosition, error) {
	var feed gtfsRTFeed
	if err := json.Unmarshal(data, &feed); err != nil {
		return nil, err
	}

	positions := make([]RawPosition, 0, len(feed.Entity))
	for _, e := range feed.Entity {
		if e.Vehicle == nil || e.Vehicle.Position == nil {
			continue
		}
		v := e.Vehicle
		positions = append(positions, RawPosition{
			VehicleID:     v.Vehicle.ID,
			TripID:        v.Trip.TripID,
			RouteID:       v.Trip.RouteID,
			Lat:           v.Position.Latitude,
			Lon:           v.Position.Longitude,
			Bearing:       v.Position.Bearing,
			Speed:         v.Position.Speed,
			Timestamp:     v.Timestamp,
			CurrentStatus: v.CurrentStatus,
			NextStopID:    v.StopID,
		})
	}
	return positions, nil
}

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

		seen := make(map[string]bool)
		var routeIDs []string
		for _, ie := range a.InformedEntity {
			if ie.RouteID != "" && !seen[ie.RouteID] {
				routeIDs = append(routeIDs, ie.RouteID)
				seen[ie.RouteID] = true
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
	}
	return updates, nil
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

// --- Enrichment ---

func EnrichPositions(raw []RawPosition, lookup RouteLookup) []models.VehiclePosition {
	out := make([]models.VehiclePosition, len(raw))
	for i, rp := range raw {
		vp := models.VehiclePosition{
			VehicleID:     rp.VehicleID,
			TripID:        rp.TripID,
			RouteID:       rp.RouteID,
			Lat:           rp.Lat,
			Lon:           rp.Lon,
			Bearing:       rp.Bearing,
			Speed:         rp.Speed,
			Timestamp:     rp.Timestamp,
			CurrentStatus: rp.CurrentStatus,
			NextStopID:    rp.NextStopID,
		}
		if route, ok := lookup.GetRoute(rp.RouteID); ok {
			vp.RouteName = route.LongName
			vp.RouteColor = route.Color
			vp.RouteType = route.Type
		}
		out[i] = vp
	}
	return out
}

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

// --- Pollers ---

func StartPositionPoller(ctx context.Context, fetcher Fetcher, lookup RouteLookup, cache *RealtimeCache, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		fetchAndCachePositions(ctx, fetcher, lookup, cache)
		for {
			select {
			case <-ctx.Done():
				slog.Info("position poller stopped")
				return
			case <-ticker.C:
				fetchAndCachePositions(ctx, fetcher, lookup, cache)
			}
		}
	}()
}

func StartAlertPoller(ctx context.Context, fetcher Fetcher, lookup RouteLookup, cache *RealtimeCache, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		fetchAndCacheAlerts(ctx, fetcher, lookup, cache)
		for {
			select {
			case <-ctx.Done():
				slog.Info("alert poller stopped")
				return
			case <-ticker.C:
				fetchAndCacheAlerts(ctx, fetcher, lookup, cache)
			}
		}
	}()
}

func StartTripUpdatePoller(ctx context.Context, fetcher Fetcher, cache *RealtimeCache, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		fetchAndCacheTripUpdates(ctx, fetcher, cache)
		for {
			select {
			case <-ctx.Done():
				slog.Info("trip update poller stopped")
				return
			case <-ticker.C:
				fetchAndCacheTripUpdates(ctx, fetcher, cache)
			}
		}
	}()
}

func fetchAndCachePositions(ctx context.Context, fetcher Fetcher, lookup RouteLookup, cache *RealtimeCache) {
	data, err := fetcher.Fetch(ctx, "/Gtfs/Feed/VehiclePosition")
	if err != nil {
		slog.Error("fetching vehicle positions", "error", err)
		return
	}
	raw, err := ParsePositions(data)
	if err != nil {
		slog.Error("parsing vehicle positions", "error", err)
		return
	}
	enriched := EnrichPositions(raw, lookup)
	cache.SetPositions(enriched)
	slog.Info("vehicle positions updated", "count", len(enriched))
}

func fetchAndCacheAlerts(ctx context.Context, fetcher Fetcher, lookup RouteLookup, cache *RealtimeCache) {
	data, err := fetcher.Fetch(ctx, "/Gtfs/Feed/Alerts")
	if err != nil {
		slog.Error("fetching alerts", "error", err)
		return
	}
	raw, err := ParseAlerts(data)
	if err != nil {
		slog.Error("parsing alerts", "error", err)
		return
	}
	enriched := EnrichAlerts(raw, lookup)
	cache.SetAlerts(enriched)
	slog.Info("alerts updated", "count", len(enriched))
}

func fetchAndCacheTripUpdates(ctx context.Context, fetcher Fetcher, cache *RealtimeCache) {
	data, err := fetcher.Fetch(ctx, "/Gtfs/Feed/TripUpdates")
	if err != nil {
		slog.Error("fetching trip updates", "error", err)
		return
	}
	updates, err := ParseTripUpdates(data)
	if err != nil {
		slog.Error("parsing trip updates", "error", err)
		return
	}
	cache.SetTripUpdates(updates)
	slog.Info("trip updates refreshed", "count", len(updates))
}
