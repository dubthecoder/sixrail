package gtfs

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/jamespfennell/gtfs"

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

// RawPosition holds pre-enrichment vehicle position data parsed from GTFS-RT.
type RawPosition struct {
	VehicleID string
	TripID    string
	RouteID   string
	Lat       float64
	Lon       float64
	Bearing   float32
	Speed     float32
	Timestamp int64
}

// RawAlert holds pre-enrichment alert data parsed from GTFS-RT.
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

// RealtimeCache is a thread-safe store for enriched positions and alerts.
type RealtimeCache struct {
	mu        sync.RWMutex
	positions []models.VehiclePosition
	alerts    []models.Alert
}

// NewRealtimeCache creates an empty RealtimeCache.
func NewRealtimeCache() *RealtimeCache {
	return &RealtimeCache{}
}

// SetPositions replaces all cached positions.
func (rc *RealtimeCache) SetPositions(positions []models.VehiclePosition) {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	rc.positions = positions
}

// GetPositions returns a copy of all cached positions.
func (rc *RealtimeCache) GetPositions() []models.VehiclePosition {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	out := make([]models.VehiclePosition, len(rc.positions))
	copy(out, rc.positions)
	return out
}

// SetAlerts replaces all cached alerts.
func (rc *RealtimeCache) SetAlerts(alerts []models.Alert) {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	rc.alerts = alerts
}

// GetAlerts returns a copy of all cached alerts.
func (rc *RealtimeCache) GetAlerts() []models.Alert {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	out := make([]models.Alert, len(rc.alerts))
	copy(out, rc.alerts)
	return out
}

// ParsePositions parses a GTFS-RT protobuf into raw vehicle positions.
func ParsePositions(data []byte) ([]RawPosition, error) {
	rt, err := gtfs.ParseRealtime(data, &gtfs.ParseRealtimeOptions{})
	if err != nil {
		return nil, err
	}

	positions := make([]RawPosition, 0, len(rt.Vehicles))
	for i := range rt.Vehicles {
		v := &rt.Vehicles[i]
		vid := v.GetID()
		trip := v.GetTrip()

		var lat, lon float64
		var bearing, speed float32
		if v.Position != nil {
			if v.Position.Latitude != nil {
				lat = float64(*v.Position.Latitude)
			}
			if v.Position.Longitude != nil {
				lon = float64(*v.Position.Longitude)
			}
			if v.Position.Bearing != nil {
				bearing = *v.Position.Bearing
			}
			if v.Position.Speed != nil {
				speed = *v.Position.Speed
			}
		}

		var ts int64
		if v.Timestamp != nil {
			ts = v.Timestamp.Unix()
		}

		positions = append(positions, RawPosition{
			VehicleID: vid.ID,
			TripID:    trip.ID.ID,
			RouteID:   trip.ID.RouteID,
			Lat:       lat,
			Lon:       lon,
			Bearing:   bearing,
			Speed:     speed,
			Timestamp: ts,
		})
	}
	return positions, nil
}

// ParseAlerts parses a GTFS-RT protobuf into raw alerts.
func ParseAlerts(data []byte) ([]RawAlert, error) {
	rt, err := gtfs.ParseRealtime(data, &gtfs.ParseRealtimeOptions{})
	if err != nil {
		return nil, err
	}

	alerts := make([]RawAlert, 0, len(rt.Alerts))
	for i := range rt.Alerts {
		a := &rt.Alerts[i]

		var headline, description, url string
		if len(a.Header) > 0 {
			headline = a.Header[0].Text
		}
		if len(a.Description) > 0 {
			description = a.Description[0].Text
		}
		if len(a.URL) > 0 {
			url = a.URL[0].Text
		}

		var routeIDs []string
		seen := make(map[string]bool)
		for _, ie := range a.InformedEntities {
			if ie.RouteID != nil && !seen[*ie.RouteID] {
				routeIDs = append(routeIDs, *ie.RouteID)
				seen[*ie.RouteID] = true
			}
		}

		var startTime, endTime int64
		if len(a.ActivePeriods) > 0 {
			if a.ActivePeriods[0].StartsAt != nil {
				startTime = a.ActivePeriods[0].StartsAt.Unix()
			}
			if a.ActivePeriods[0].EndsAt != nil {
				endTime = a.ActivePeriods[0].EndsAt.Unix()
			}
		}

		alerts = append(alerts, RawAlert{
			ID:          a.ID,
			Effect:      a.Effect.String(),
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

// EnrichPositions adds route names and colors from a RouteLookup.
func EnrichPositions(raw []RawPosition, lookup RouteLookup) []models.VehiclePosition {
	out := make([]models.VehiclePosition, len(raw))
	for i, rp := range raw {
		vp := models.VehiclePosition{
			VehicleID: rp.VehicleID,
			TripID:    rp.TripID,
			RouteID:   rp.RouteID,
			Lat:       rp.Lat,
			Lon:       rp.Lon,
			Bearing:   rp.Bearing,
			Speed:     rp.Speed,
			Timestamp: rp.Timestamp,
		}
		if route, ok := lookup.GetRoute(rp.RouteID); ok {
			vp.RouteName = route.LongName
			vp.RouteColor = route.Color
		}
		out[i] = vp
	}
	return out
}

// EnrichAlerts adds route names from a RouteLookup.
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

// StartPositionPoller launches a background goroutine that periodically
// fetches GTFS-RT vehicle positions, enriches them, and stores them in cache.
func StartPositionPoller(ctx context.Context, fetcher Fetcher, lookup RouteLookup, cache *RealtimeCache, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		// Fetch immediately on start.
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

// StartAlertPoller launches a background goroutine that periodically
// fetches GTFS-RT alerts, enriches them, and stores them in cache.
func StartAlertPoller(ctx context.Context, fetcher Fetcher, lookup RouteLookup, cache *RealtimeCache, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		// Fetch immediately on start.
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
