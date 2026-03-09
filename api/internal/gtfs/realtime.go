package gtfs

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/teclara/railsix/api/internal/models"
)

// Fetcher retrieves raw bytes from a remote path.
type Fetcher interface {
	Fetch(ctx context.Context, path string) ([]byte, error)
}

// RouteLookup is satisfied by StaticStore.
type RouteLookup interface {
	GetRoute(id string) (models.Route, bool)
}

// ServiceGlanceFetcher fetches service-at-a-glance and exceptions data.
type ServiceGlanceFetcher interface {
	GetServiceGlance(ctx context.Context) ([]models.ServiceGlanceEntry, error)
	GetExceptions(ctx context.Context) (map[string]bool, error)
}

// --- JSON structures matching Metrolinx GTFS-RT JSON format ---

type gtfsRTFeed struct {
	Entity []gtfsRTEntity `json:"entity"`
}

type gtfsRTEntity struct {
	ID         string           `json:"id"`
	Alert      *gtfsRTAlert     `json:"alert"`
	TripUpdate *gtfsRTTripUpd   `json:"trip_update"`
	Vehicle    *gtfsRTVehicle   `json:"vehicle"`
}

type gtfsRTVehicle struct {
	Trip gtfsRTTrip `json:"trip"`
}

type gtfsRTTrip struct {
	TripID               string `json:"trip_id"`
	RouteID              string `json:"route_id"`
	ScheduleRelationship string `json:"schedule_relationship"`
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

// UnionDeparturesFetcher fetches the Union Station departures board.
type UnionDeparturesFetcher interface {
	GetUnionDepartures(ctx context.Context) ([]models.UnionDeparture, error)
}

// ttlEntry is a generic time-stamped cache entry.
type ttlEntry[T any] struct {
	data      T
	fetchedAt time.Time
}

// RealtimeCache is a thread-safe store for enriched positions, alerts, trip updates,
// service glance data, exception/cancellation info, and TTL caches.
type RealtimeCache struct {
	mu               sync.RWMutex
	alerts           []models.Alert
	tripUpdates      map[string]RawTripUpdate
	serviceGlance    map[string]models.ServiceGlanceEntry // keyed by trip number
	cancelledTrips   map[string]bool                       // set of cancelled trip numbers
	unionDepartures []models.UnionDeparture
	nextService     map[string]ttlEntry[[]models.NextServiceLine] // keyed by stopCode, 30s TTL
	fares            map[string]ttlEntry[[]models.FareInfo]         // keyed by "from|to", 1h TTL

	// Staleness tracking
	alertsUpdatedAt        time.Time
	tripUpdatesUpdatedAt   time.Time
	serviceGlanceUpdatedAt time.Time
	cancelledTripsUpdatedAt time.Time
	unionDepsUpdatedAt     time.Time
}

func NewRealtimeCache() *RealtimeCache {
	return &RealtimeCache{
		tripUpdates:     make(map[string]RawTripUpdate),
		serviceGlance:   make(map[string]models.ServiceGlanceEntry),
		cancelledTrips: make(map[string]bool),
		nextService:    make(map[string]ttlEntry[[]models.NextServiceLine]),
		fares:           make(map[string]ttlEntry[[]models.FareInfo]),
	}
}

func (rc *RealtimeCache) SetAlerts(alerts []models.Alert) {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	rc.alerts = alerts
	rc.alertsUpdatedAt = time.Now()
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
	rc.tripUpdatesUpdatedAt = time.Now()
}

func (rc *RealtimeCache) GetTripUpdate(tripID string) (RawTripUpdate, bool) {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	u, ok := rc.tripUpdates[tripID]
	return u, ok
}

func (rc *RealtimeCache) SetServiceGlance(entries map[string]models.ServiceGlanceEntry) {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	rc.serviceGlance = entries
	rc.serviceGlanceUpdatedAt = time.Now()
}

// GetServiceGlanceEntry returns the service glance data for a trip number.
func (rc *RealtimeCache) GetServiceGlanceEntry(tripNumber string) (models.ServiceGlanceEntry, bool) {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	e, ok := rc.serviceGlance[tripNumber]
	return e, ok
}

// GetAllServiceGlance returns all service glance entries (for network health aggregation).
func (rc *RealtimeCache) GetAllServiceGlance() map[string]models.ServiceGlanceEntry {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	out := make(map[string]models.ServiceGlanceEntry, len(rc.serviceGlance))
	for k, v := range rc.serviceGlance {
		out[k] = v
	}
	return out
}

func (rc *RealtimeCache) SetCancelledTrips(cancelled map[string]bool) {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	rc.cancelledTrips = cancelled
	rc.cancelledTripsUpdatedAt = time.Now()
}

func (rc *RealtimeCache) IsTripCancelled(tripNumber string) bool {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	return rc.cancelledTrips[tripNumber]
}

// --- Union departures cache ---

func (rc *RealtimeCache) SetUnionDepartures(deps []models.UnionDeparture) {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	rc.unionDepartures = deps
	rc.unionDepsUpdatedAt = time.Now()
}

func (rc *RealtimeCache) GetUnionDepartures() []models.UnionDeparture {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	out := make([]models.UnionDeparture, len(rc.unionDepartures))
	copy(out, rc.unionDepartures)
	return out
}

// --- Staleness tracking ---

const maxCacheAge = 5 * time.Minute

// CacheStatus reports the age of each polled data source and whether any are stale.
type CacheStatus struct {
	AlertsAge        time.Duration `json:"alertsAge"`
	TripUpdatesAge   time.Duration `json:"tripUpdatesAge"`
	ServiceGlanceAge time.Duration `json:"serviceGlanceAge"`
	UnionDepsAge     time.Duration `json:"unionDepsAge"`
	Stale            bool          `json:"stale"`
}

func (rc *RealtimeCache) Status() CacheStatus {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	now := time.Now()
	s := CacheStatus{}
	if !rc.alertsUpdatedAt.IsZero() {
		s.AlertsAge = now.Sub(rc.alertsUpdatedAt)
	}
	if !rc.tripUpdatesUpdatedAt.IsZero() {
		s.TripUpdatesAge = now.Sub(rc.tripUpdatesUpdatedAt)
	}
	if !rc.serviceGlanceUpdatedAt.IsZero() {
		s.ServiceGlanceAge = now.Sub(rc.serviceGlanceUpdatedAt)
	}
	if !rc.unionDepsUpdatedAt.IsZero() {
		s.UnionDepsAge = now.Sub(rc.unionDepsUpdatedAt)
	}
	s.Stale = (s.AlertsAge > maxCacheAge && !rc.alertsUpdatedAt.IsZero()) ||
		(s.TripUpdatesAge > maxCacheAge && !rc.tripUpdatesUpdatedAt.IsZero()) ||
		(s.ServiceGlanceAge > maxCacheAge && !rc.serviceGlanceUpdatedAt.IsZero()) ||
		(s.UnionDepsAge > maxCacheAge && !rc.unionDepsUpdatedAt.IsZero())
	return s
}

func (rc *RealtimeCache) clearStaleAlerts() {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	if !rc.alertsUpdatedAt.IsZero() && time.Since(rc.alertsUpdatedAt) > maxCacheAge {
		slog.Warn("clearing stale alerts cache", "age", time.Since(rc.alertsUpdatedAt))
		rc.alerts = nil
		rc.alertsUpdatedAt = time.Time{}
	}
}

func (rc *RealtimeCache) clearStaleTripUpdates() {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	if !rc.tripUpdatesUpdatedAt.IsZero() && time.Since(rc.tripUpdatesUpdatedAt) > maxCacheAge {
		slog.Warn("clearing stale trip updates cache", "age", time.Since(rc.tripUpdatesUpdatedAt))
		rc.tripUpdates = make(map[string]RawTripUpdate)
		rc.tripUpdatesUpdatedAt = time.Time{}
	}
}

func (rc *RealtimeCache) clearStaleServiceGlance() {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	if !rc.serviceGlanceUpdatedAt.IsZero() && time.Since(rc.serviceGlanceUpdatedAt) > maxCacheAge {
		slog.Warn("clearing stale service glance cache", "age", time.Since(rc.serviceGlanceUpdatedAt))
		rc.serviceGlance = make(map[string]models.ServiceGlanceEntry)
		rc.serviceGlanceUpdatedAt = time.Time{}
	}
}

func (rc *RealtimeCache) clearStaleCancelledTrips() {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	if !rc.cancelledTripsUpdatedAt.IsZero() && time.Since(rc.cancelledTripsUpdatedAt) > maxCacheAge {
		slog.Warn("clearing stale cancelled trips cache", "age", time.Since(rc.cancelledTripsUpdatedAt))
		rc.cancelledTrips = make(map[string]bool)
		rc.cancelledTripsUpdatedAt = time.Time{}
	}
}

func (rc *RealtimeCache) clearStaleUnionDepartures() {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	if !rc.unionDepsUpdatedAt.IsZero() && time.Since(rc.unionDepsUpdatedAt) > maxCacheAge {
		slog.Warn("clearing stale union departures cache", "age", time.Since(rc.unionDepsUpdatedAt))
		rc.unionDepartures = nil
		rc.unionDepsUpdatedAt = time.Time{}
	}
}

const (
	maxNextServiceEntries = 500
	maxFaresEntries       = 200
)

// --- NextService TTL cache (30s) ---

func (rc *RealtimeCache) GetNextService(stopCode string) ([]models.NextServiceLine, bool) {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	entry, ok := rc.nextService[stopCode]
	if !ok || time.Since(entry.fetchedAt) > 30*time.Second {
		return nil, false
	}
	return entry.data, true
}

func (rc *RealtimeCache) SetNextService(stopCode string, lines []models.NextServiceLine) {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	rc.nextService[stopCode] = ttlEntry[[]models.NextServiceLine]{data: lines, fetchedAt: time.Now()}
	// Evict stale entries
	for k, v := range rc.nextService {
		if time.Since(v.fetchedAt) > 5*time.Minute {
			delete(rc.nextService, k)
		}
	}
	// Cap max size by removing oldest entries
	for len(rc.nextService) > maxNextServiceEntries {
		oldestKey := ""
		var oldestTime time.Time
		for k, v := range rc.nextService {
			if oldestKey == "" || v.fetchedAt.Before(oldestTime) {
				oldestKey = k
				oldestTime = v.fetchedAt
			}
		}
		delete(rc.nextService, oldestKey)
	}
}

// --- Fares TTL cache (1h) ---

func (rc *RealtimeCache) GetFares(from, to string) ([]models.FareInfo, bool) {
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	entry, ok := rc.fares[from+"|"+to]
	if !ok || time.Since(entry.fetchedAt) > time.Hour {
		return nil, false
	}
	return entry.data, true
}

func (rc *RealtimeCache) SetFares(from, to string, fares []models.FareInfo) {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	rc.fares[from+"|"+to] = ttlEntry[[]models.FareInfo]{data: fares, fetchedAt: time.Now()}
	// Evict stale entries
	for k, v := range rc.fares {
		if time.Since(v.fetchedAt) > 2*time.Hour {
			delete(rc.fares, k)
		}
	}
	// Cap max size by removing oldest entries
	for len(rc.fares) > maxFaresEntries {
		oldestKey := ""
		var oldestTime time.Time
		for k, v := range rc.fares {
			if oldestKey == "" || v.fetchedAt.Before(oldestTime) {
				oldestKey = k
				oldestTime = v.fetchedAt
			}
		}
		delete(rc.fares, oldestKey)
	}
}

// --- JSON parsers ---

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
		// Also index by trip number (last segment) so lookups work
		// regardless of which date-prefix the static schedule uses.
		if idx := strings.LastIndex(tripID, "-"); idx >= 0 && idx+1 < len(tripID) {
			tripNum := tripID[idx+1:]
			if _, exists := updates[tripNum]; !exists {
				updates[tripNum] = raw
			}
		}
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

func fetchAndCacheAlerts(ctx context.Context, fetcher Fetcher, lookup RouteLookup, cache *RealtimeCache) {
	data, err := fetcher.Fetch(ctx, "/Gtfs/Feed/Alerts")
	if err != nil {
		slog.Error("fetching alerts", "error", err)
		cache.clearStaleAlerts()
		return
	}
	raw, err := ParseAlerts(data)
	if err != nil {
		slog.Error("parsing alerts", "error", err)
		cache.clearStaleAlerts()
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
		cache.clearStaleTripUpdates()
		return
	}
	updates, err := ParseTripUpdates(data)
	if err != nil {
		slog.Error("parsing trip updates", "error", err)
		cache.clearStaleTripUpdates()
		return
	}
	cache.SetTripUpdates(updates)
	slog.Info("trip updates refreshed", "count", len(updates))
}

// StartServiceGlancePoller polls ServiceataGlance/Trains/All for occupancy, car count, and line data.
func StartServiceGlancePoller(ctx context.Context, fetcher ServiceGlanceFetcher, cache *RealtimeCache, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		fetchAndCacheServiceGlance(ctx, fetcher, cache)
		for {
			select {
			case <-ctx.Done():
				slog.Info("service glance poller stopped")
				return
			case <-ticker.C:
				fetchAndCacheServiceGlance(ctx, fetcher, cache)
			}
		}
	}()
}

// StartExceptionsPoller polls ServiceUpdate/Exceptions/All for cancelled trips.
func StartExceptionsPoller(ctx context.Context, fetcher ServiceGlanceFetcher, cache *RealtimeCache, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		fetchAndCacheExceptions(ctx, fetcher, cache)
		for {
			select {
			case <-ctx.Done():
				slog.Info("exceptions poller stopped")
				return
			case <-ticker.C:
				fetchAndCacheExceptions(ctx, fetcher, cache)
			}
		}
	}()
}

func fetchAndCacheServiceGlance(ctx context.Context, fetcher ServiceGlanceFetcher, cache *RealtimeCache) {
	entries, err := fetcher.GetServiceGlance(ctx)
	if err != nil {
		slog.Error("fetching service glance", "error", err)
		cache.clearStaleServiceGlance()
		return
	}
	m := make(map[string]models.ServiceGlanceEntry, len(entries))
	for _, e := range entries {
		m[e.TripNumber] = e
	}
	cache.SetServiceGlance(m)
	slog.Info("service glance updated", "count", len(entries))
}

func fetchAndCacheExceptions(ctx context.Context, fetcher ServiceGlanceFetcher, cache *RealtimeCache) {
	cancelled, err := fetcher.GetExceptions(ctx)
	if err != nil {
		slog.Error("fetching exceptions", "error", err)
		cache.clearStaleCancelledTrips()
		return
	}
	cache.SetCancelledTrips(cancelled)
	slog.Info("exceptions updated", "cancelledTrips", len(cancelled))
}

// StartUnionDeparturesPoller polls the Union Station departures board.
func StartUnionDeparturesPoller(ctx context.Context, fetcher UnionDeparturesFetcher, cache *RealtimeCache, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		fetchAndCacheUnionDepartures(ctx, fetcher, cache)
		for {
			select {
			case <-ctx.Done():
				slog.Info("union departures poller stopped")
				return
			case <-ticker.C:
				fetchAndCacheUnionDepartures(ctx, fetcher, cache)
			}
		}
	}()
}

func fetchAndCacheUnionDepartures(ctx context.Context, fetcher UnionDeparturesFetcher, cache *RealtimeCache) {
	deps, err := fetcher.GetUnionDepartures(ctx)
	if err != nil {
		slog.Error("fetching union departures", "error", err)
		cache.clearStaleUnionDepartures()
		return
	}
	cache.SetUnionDepartures(deps)
	slog.Info("union departures updated", "count", len(deps))
}

