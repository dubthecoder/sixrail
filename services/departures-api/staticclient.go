package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/teclara/railsix/shared/models"
)

// ScheduledDeparture mirrors gtfs-static's store.ScheduledDeparture.
type ScheduledDeparture struct {
	TripID        string `json:"tripId"`
	RouteID       string `json:"routeId"`
	ServiceID     string `json:"serviceId"`
	Headsign      string `json:"headsign"`
	DepartureTime int64  `json:"departureTime"` // nanoseconds from midnight
}

// TripStop mirrors gtfs-static's store.TripStop.
type TripStop struct {
	StopID        string `json:"stopId"`
	ArrivalTime   int64  `json:"arrivalTime"`   // nanoseconds from midnight
	DepartureTime int64  `json:"departureTime"` // nanoseconds from midnight
}

// TripInfo mirrors gtfs-static's store.TripInfo.
type TripInfo struct {
	TripID    string     `json:"tripId"`
	RouteID   string     `json:"routeId"`
	ServiceID string     `json:"serviceId"`
	Stops     []TripStop `json:"stops"`
}

// ArrivalResult mirrors gtfs-static's store.ArrivalResult.
type ArrivalResult struct {
	Duration int64 `json:"duration"` // nanoseconds
	OK       bool  `json:"ok"`
}

// StaticClient is an HTTP client for the gtfs-static microservice with
// in-memory caching for data that doesn't change within a GTFS refresh cycle.
type StaticClient struct {
	baseURL string
	client  *http.Client

	routeCache   sync.Map // routeID → models.Route
	tripCache    sync.Map // tripID → TripInfo
	nameCache    sync.Map // stopID → string
	serviceCache sync.Map // "serviceID|YYYY-MM-DD" → bool
}

// NewStaticClient creates a StaticClient pointing at the given base URL.
func NewStaticClient(baseURL string) *StaticClient {
	return &StaticClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

func (sc *StaticClient) get(path string) ([]byte, error) {
	resp, err := sc.client.Get(sc.baseURL + path)
	if err != nil {
		return nil, fmt.Errorf("static client GET %s: %w", path, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("static client GET %s: status %d", path, resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

// ScheduleCandidate mirrors gtfs-static's store.ScheduleCandidate.
type ScheduleCandidate struct {
	TripID         string   `json:"tripId"`
	TripNumber     string   `json:"tripNumber"`
	RouteShortName string   `json:"routeShortName"`
	RouteLongName  string   `json:"routeLongName"`
	RouteColor     string   `json:"routeColor"`
	RouteType      int      `json:"routeType"`
	Headsign       string   `json:"headsign"`
	ScheduledTime  string   `json:"scheduledTime"`
	Platform       string   `json:"platform"`
	Stops          []string `json:"stops"`
	LastStopID     string   `json:"lastStopId"`
	IsExpress      bool     `json:"isExpress"`
	StopID         string   `json:"stopId"`
	DepartureNano  int64    `json:"departureNano"`
	ServiceDay     string   `json:"serviceDay"`
}

// GetStops proxies the /stops endpoint from gtfs-static, returning raw JSON.
func (sc *StaticClient) GetStops() ([]byte, error) {
	return sc.get("/stops")
}

// Ready checks whether gtfs-static has finished loading its data.
func (sc *StaticClient) Ready(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, sc.baseURL+"/ready", nil)
	if err != nil {
		return fmt.Errorf("build gtfs-static readiness request: %w", err)
	}

	resp, err := sc.client.Do(req)
	if err != nil {
		return fmt.Errorf("gtfs-static readiness request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("gtfs-static readiness returned %d", resp.StatusCode)
	}

	return nil
}

// GetSchedule returns pre-filtered departure candidates for a stop code.
// All filtering (last stop, service active, time window, dedup) is done server-side.
func (sc *StaticClient) GetSchedule(code string, now time.Time) ([]ScheduleCandidate, error) {
	path := "/schedule/" + url.PathEscape(code) + "?now=" + fmt.Sprintf("%d", now.Unix())
	data, err := sc.get(path)
	if err != nil {
		return nil, err
	}
	var candidates []ScheduleCandidate
	if err := json.Unmarshal(data, &candidates); err != nil {
		return nil, fmt.Errorf("decode schedule: %w", err)
	}
	return candidates, nil
}

// StopIDsForCode returns all stop IDs for a stop code.
func (sc *StaticClient) StopIDsForCode(code string) ([]string, error) {
	data, err := sc.get("/stops/" + url.PathEscape(code) + "/ids")
	if err != nil {
		return nil, err
	}
	var ids []string
	if err := json.Unmarshal(data, &ids); err != nil {
		return nil, fmt.Errorf("decode stop ids: %w", err)
	}
	return ids, nil
}

// DeparturesForStop returns scheduled departures for a stop ID.
func (sc *StaticClient) DeparturesForStop(stopID string) ([]ScheduledDeparture, error) {
	data, err := sc.get("/departures/" + url.PathEscape(stopID))
	if err != nil {
		return nil, err
	}
	var deps []ScheduledDeparture
	if err := json.Unmarshal(data, &deps); err != nil {
		return nil, fmt.Errorf("decode departures: %w", err)
	}
	return deps, nil
}

// getTrip returns trip info, using the in-memory cache.
func (sc *StaticClient) getTrip(tripID string) (TripInfo, bool) {
	if v, ok := sc.tripCache.Load(tripID); ok {
		return v.(TripInfo), true
	}
	data, err := sc.get("/trips/" + url.PathEscape(tripID))
	if err != nil {
		return TripInfo{}, false
	}
	var trip TripInfo
	if err := json.Unmarshal(data, &trip); err != nil {
		return TripInfo{}, false
	}
	sc.tripCache.Store(tripID, trip)
	return trip, true
}

// IsLastStop returns true if any of the given stop IDs is the final stop of the trip.
// Computed locally from cached trip data.
func (sc *StaticClient) IsLastStop(tripID string, stopIDs []string) (bool, error) {
	trip, ok := sc.getTrip(tripID)
	if !ok {
		return false, fmt.Errorf("trip not found: %s", tripID)
	}
	if len(trip.Stops) == 0 {
		return false, nil
	}
	lastStopID := trip.Stops[len(trip.Stops)-1].StopID
	for _, id := range stopIDs {
		if id == lastStopID {
			return true, nil
		}
	}
	return false, nil
}

// IsServiceActive returns whether a service is active on a given date.
// Cached by serviceID+date since this doesn't change within a day.
func (sc *StaticClient) IsServiceActive(serviceID string, date time.Time) (bool, error) {
	key := serviceID + "|" + date.Format("2006-01-02")
	if v, ok := sc.serviceCache.Load(key); ok {
		return v.(bool), nil
	}
	dateStr := date.Format("2006-01-02")
	data, err := sc.get("/services/" + url.PathEscape(serviceID) + "/active?date=" + dateStr)
	if err != nil {
		return false, err
	}
	var result struct {
		Active bool `json:"active"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return false, fmt.Errorf("decode service active: %w", err)
	}
	sc.serviceCache.Store(key, result.Active)
	return result.Active, nil
}

// GetRoute returns route info for a route ID. Cached permanently.
func (sc *StaticClient) GetRoute(routeID string) (models.Route, bool) {
	if v, ok := sc.routeCache.Load(routeID); ok {
		return v.(models.Route), true
	}
	data, err := sc.get("/routes/" + url.PathEscape(routeID))
	if err != nil {
		return models.Route{}, false
	}
	var route models.Route
	if err := json.Unmarshal(data, &route); err != nil {
		return models.Route{}, false
	}
	sc.routeCache.Store(routeID, route)
	return route, true
}

// GetStopName returns the name for a stop ID. Cached permanently.
func (sc *StaticClient) GetStopName(stopID string) (string, error) {
	if v, ok := sc.nameCache.Load(stopID); ok {
		return v.(string), nil
	}
	data, err := sc.get("/stop-name/" + url.PathEscape(stopID))
	if err != nil {
		return "", err
	}
	var result struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return "", fmt.Errorf("decode stop name: %w", err)
	}
	sc.nameCache.Store(stopID, result.Name)
	return result.Name, nil
}

// RemainingStopNames returns stop names after the departure stop in a trip.
// Computed locally from cached trip and stop name data.
func (sc *StaticClient) RemainingStopNames(tripID string, stopIDs []string) ([]string, error) {
	trip, ok := sc.getTrip(tripID)
	if !ok {
		return nil, fmt.Errorf("trip not found: %s", tripID)
	}
	depSet := make(map[string]bool, len(stopIDs))
	for _, id := range stopIDs {
		depSet[id] = true
	}
	found := false
	var names []string
	for _, ts := range trip.Stops {
		if !found {
			if depSet[ts.StopID] {
				found = true
			}
			continue
		}
		name, _ := sc.GetStopName(ts.StopID)
		if name != "" {
			names = append(names, name)
		}
	}
	return names, nil
}

// IsExpress returns whether a trip is express (skips stops).
// Computed locally from cached trip data — compares stop count to the max
// for the same route+origin+destination.
func (sc *StaticClient) IsExpress(tripID string) (bool, error) {
	// Fetch from gtfs-static which has the maxRouteStops index.
	data, err := sc.get("/trips/" + url.PathEscape(tripID) + "/is-express")
	if err != nil {
		return false, err
	}
	var result struct {
		IsExpress bool `json:"isExpress"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return false, fmt.Errorf("decode is-express: %w", err)
	}
	return result.IsExpress, nil
}

// ArrivalTimeAtStop returns the arrival duration at a destination stop.
// Computed locally from cached trip data.
func (sc *StaticClient) ArrivalTimeAtStop(tripID string, destIDs, originIDs []string) (ArrivalResult, error) {
	trip, ok := sc.getTrip(tripID)
	if !ok {
		return ArrivalResult{}, fmt.Errorf("trip not found: %s", tripID)
	}
	destSet := make(map[string]bool, len(destIDs))
	for _, id := range destIDs {
		destSet[id] = true
	}
	startIdx := 0
	if len(originIDs) > 0 {
		originSet := make(map[string]bool, len(originIDs))
		for _, id := range originIDs {
			originSet[id] = true
		}
		found := false
		for i, ts := range trip.Stops {
			if originSet[ts.StopID] {
				startIdx = i + 1
				found = true
				break
			}
		}
		if !found {
			return ArrivalResult{OK: false}, nil
		}
	}
	for i := startIdx; i < len(trip.Stops); i++ {
		if destSet[trip.Stops[i].StopID] {
			return ArrivalResult{Duration: trip.Stops[i].ArrivalTime, OK: true}, nil
		}
	}
	return ArrivalResult{OK: false}, nil
}
