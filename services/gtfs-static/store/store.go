package store

import (
	"fmt"
	"log/slog"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/jamespfennell/gtfs"

	"github.com/teclara/railsix/shared/models"
)

// ScheduledDeparture is a single stop-time entry in the schedule index.
type ScheduledDeparture struct {
	TripID        string `json:"tripId"`
	RouteID       string `json:"routeId"`
	ServiceID     string `json:"serviceId"`
	Headsign      string `json:"headsign"`
	DepartureTime int64  `json:"departureTime"` // nanoseconds from midnight (local time)
}

// TripStop is one stop in a trip's ordered sequence.
type TripStop struct {
	StopID        string `json:"stopId"`
	ArrivalTime   int64  `json:"arrivalTime"`   // nanoseconds from midnight of service day
	DepartureTime int64  `json:"departureTime"` // nanoseconds from midnight of service day
}

// TripInfo holds a trip's identity and full stop sequence for departure enrichment.
type TripInfo struct {
	TripID    string     `json:"tripId"`
	RouteID   string     `json:"routeId"`
	ServiceID string     `json:"serviceId"`
	Stops     []TripStop `json:"stops"`
}

// ArrivalResult is the JSON-serializable result of an arrival time query.
type ArrivalResult struct {
	Duration int64 `json:"duration"` // nanoseconds
	OK       bool  `json:"ok"`
}

// StaticStore holds parsed GTFS static data with thread-safe access.
type StaticStore struct {
	mu        sync.RWMutex
	loaded    bool // true once GTFS data has been successfully loaded at least once
	stops     []models.Stop
	stopNames map[string]string // stopID → name
	routes    map[string]models.Route
	stopIndex map[string][]ScheduledDeparture // stopID → sorted departures
	stopCodes map[string][]string             // stopCode → []stopID (parent + children)
	services  map[string]gtfs.Service         // serviceID → service
	tripIndex     map[string]TripInfo // tripID → TripInfo
	maxRouteStops map[string]int      // routeID|firstStop|lastStop → max stop count
}

// NewStaticStore creates a StaticStore and loads the given GTFS ZIP data.
func NewStaticStore(zipData []byte) (*StaticStore, error) {
	s := &StaticStore{}
	if err := s.load(zipData); err != nil {
		return nil, err
	}
	return s, nil
}

// NewEmptyStaticStore creates a StaticStore with no data loaded.
func NewEmptyStaticStore() *StaticStore {
	return &StaticStore{}
}

// Ready reports whether the store has been successfully loaded at least once.
func (s *StaticStore) Ready() bool {
	if s == nil {
		return false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.loaded
}

func (s *StaticStore) load(zipData []byte) error {
	static, err := gtfs.ParseStatic(zipData, gtfs.ParseStaticOptions{})
	if err != nil {
		return fmt.Errorf("parsing GTFS static: %w", err)
	}

	// --- Stops ---
	stops := make([]models.Stop, 0, len(static.Stops))
	stopNames := make(map[string]string, len(static.Stops))
	for i := range static.Stops {
		gs := &static.Stops[i]
		stopNames[gs.Id] = gs.Name
		if gs.Latitude == nil || gs.Longitude == nil {
			continue
		}
		var parentID string
		if gs.Parent != nil {
			parentID = gs.Parent.Id
		}
		stops = append(stops, models.Stop{
			ID:       gs.Id,
			Code:     gs.Code,
			Name:     gs.Name,
			Lat:      *gs.Latitude,
			Lon:      *gs.Longitude,
			ParentID: parentID,
		})
	}

	// --- Routes ---
	routes := make(map[string]models.Route, len(static.Routes))
	for i := range static.Routes {
		gr := &static.Routes[i]
		routes[gr.Id] = models.Route{
			ID:        gr.Id,
			ShortName: gr.ShortName,
			LongName:  gr.LongName,
			Color:     gr.Color,
			TextColor: gr.TextColor,
			Type:      int(gr.Type),
		}
	}

	// --- Services (calendar) ---
	services := make(map[string]gtfs.Service, len(static.Services))
	for i := range static.Services {
		services[static.Services[i].Id] = static.Services[i]
	}

	// --- Stop code → stop IDs mapping ---
	stopCodes := make(map[string][]string)
	for i := range static.Stops {
		gs := &static.Stops[i]
		key := gs.Code
		if key == "" {
			key = gs.Id
		}
		stopCodes[key] = append(stopCodes[key], gs.Id)
		if gs.Parent != nil {
			parentKey := gs.Parent.Code
			if parentKey == "" {
				parentKey = gs.Parent.Id
			}
			if parentKey != "" {
				stopCodes[parentKey] = appendUnique(stopCodes[parentKey], gs.Id)
			}
		}
	}

	// --- Schedule index + trip index (single pass over trips) ---
	// Use string interner to deduplicate repeated IDs and headsigns.
	intern := newInterner()
	stopIndex := make(map[string][]ScheduledDeparture)
	tripIndex := make(map[string]TripInfo, len(static.Trips))
	for i := range static.Trips {
		trip := &static.Trips[i]
		if trip.Route == nil || trip.Service == nil {
			continue
		}
		headsign := trip.Headsign
		if headsign == "" {
			headsign = trip.Route.LongName
		}
		headsign = intern.intern(headsign)
		tripID := intern.intern(trip.ID)
		routeID := intern.intern(trip.Route.Id)
		serviceID := intern.intern(trip.Service.Id)
		tripStops := make([]TripStop, 0, len(trip.StopTimes))
		for j := range trip.StopTimes {
			st := &trip.StopTimes[j]
			if st.Stop == nil {
				continue
			}
			stopID := intern.intern(st.Stop.Id)
			stopIndex[stopID] = append(stopIndex[stopID], ScheduledDeparture{
				TripID:        tripID,
				RouteID:       routeID,
				ServiceID:     serviceID,
				Headsign:      headsign,
				DepartureTime: int64(st.DepartureTime),
			})
			tripStops = append(tripStops, TripStop{
				StopID:        stopID,
				ArrivalTime:   int64(st.ArrivalTime),
				DepartureTime: int64(st.DepartureTime),
			})
		}
		if len(tripStops) < 2 {
			continue
		}
		tripIndex[tripID] = TripInfo{
			TripID:    tripID,
			RouteID:   routeID,
			ServiceID: serviceID,
			Stops:     tripStops,
		}
	}

	// --- Max stop count per route+endpoint pair (for express detection) ---
	maxRouteStops := make(map[string]int)
	for _, ti := range tripIndex {
		if len(ti.Stops) < 2 {
			continue
		}
		key := ti.RouteID + "|" + ti.Stops[0].StopID + "|" + ti.Stops[len(ti.Stops)-1].StopID
		if len(ti.Stops) > maxRouteStops[key] {
			maxRouteStops[key] = len(ti.Stops)
		}
	}

	s.mu.Lock()
	s.stops = stops
	s.stopNames = stopNames
	s.routes = routes
	s.stopIndex = stopIndex
	s.stopCodes = stopCodes
	s.services = services
	s.tripIndex = tripIndex
	s.maxRouteStops = maxRouteStops
	s.loaded = true
	s.mu.Unlock()

	slog.Info("GTFS static loaded",
		"stops", len(stops),
		"routes", len(routes),
		"services", len(services),
		"stopIndexEntries", len(stopIndex),
	)
	return nil
}

func appendUnique(slice []string, val string) []string {
	for _, v := range slice {
		if v == val {
			return slice
		}
	}
	return append(slice, val)
}

// stringInterner deduplicates strings to reduce heap allocations.
type stringInterner map[string]string

func newInterner() stringInterner { return make(stringInterner, 4096) }

func (si stringInterner) intern(s string) string {
	if interned, ok := si[s]; ok {
		return interned
	}
	si[s] = s
	return s
}

// Refresh reloads GTFS data from the given ZIP bytes.
func (s *StaticStore) Refresh(zipData []byte) error {
	return s.load(zipData)
}

// AllStops returns a copy of all stops.
func (s *StaticStore) AllStops() []models.Stop {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]models.Stop, len(s.stops))
	copy(out, s.stops)
	return out
}

// GetStopName returns the name for a stop ID.
func (s *StaticStore) GetStopName(id string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.stopNames[id]
}

// GetTrip returns trip info for a trip ID.
func (s *StaticStore) GetTrip(tripID string) (TripInfo, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	t, ok := s.tripIndex[tripID]
	return t, ok
}

// RemainingStopNames returns the names of stops after the given stopIDs in a trip.
func (s *StaticStore) RemainingStopNames(tripID string, departureStopIDs []string) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	trip, ok := s.tripIndex[tripID]
	if !ok {
		return nil
	}

	depSet := make(map[string]bool, len(departureStopIDs))
	for _, id := range departureStopIDs {
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
		name := s.stopNames[ts.StopID]
		if name != "" {
			names = append(names, name)
		}
	}
	return names
}

// IsLastStop returns true if any of the given stop IDs is the final stop of the trip.
func (s *StaticStore) IsLastStop(tripID string, stopIDs []string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	trip, ok := s.tripIndex[tripID]
	if !ok || len(trip.Stops) == 0 {
		return false
	}
	lastStopID := trip.Stops[len(trip.Stops)-1].StopID
	for _, id := range stopIDs {
		if id == lastStopID {
			return true
		}
	}
	return false
}

// IsExpress returns true if a trip skips stops compared to the longest trip
// on the same route with the same origin and destination stops.
func (s *StaticStore) IsExpress(tripID string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	trip, ok := s.tripIndex[tripID]
	if !ok || len(trip.Stops) < 2 {
		return false
	}
	key := trip.RouteID + "|" + trip.Stops[0].StopID + "|" + trip.Stops[len(trip.Stops)-1].StopID
	max := s.maxRouteStops[key]
	return max > 0 && len(trip.Stops) < max
}

// GetRoute returns route info for a route ID.
func (s *StaticStore) GetRoute(id string) (models.Route, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	r, ok := s.routes[id]
	return r, ok
}

// StopIDsForCode returns all stop IDs (parent + children) for a given stop code.
func (s *StaticStore) StopIDsForCode(code string) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	ids := s.stopCodes[code]
	out := make([]string, len(ids))
	copy(out, ids)
	return out
}

// DeparturesForStop returns scheduled departures for a stop ID.
func (s *StaticStore) DeparturesForStop(stopID string) []ScheduledDeparture {
	s.mu.RLock()
	defer s.mu.RUnlock()
	src := s.stopIndex[stopID]
	out := make([]ScheduledDeparture, len(src))
	copy(out, src)
	return out
}

// ScheduleCandidate is a pre-filtered departure candidate returned by ScheduleForStop.
type ScheduleCandidate struct {
	TripID         string   `json:"tripId"`
	TripNumber     string   `json:"tripNumber"`
	RouteShortName string   `json:"routeShortName"`
	RouteLongName  string   `json:"routeLongName"`
	RouteColor     string   `json:"routeColor"`
	RouteType      int      `json:"routeType"`
	Headsign       string   `json:"headsign"`
	ScheduledTime  string   `json:"scheduledTime"` // "HH:MM"
	Platform       string   `json:"platform"`
	Stops          []string `json:"stops"`    // remaining stop names after departure
	IsExpress      bool     `json:"isExpress"`
	StopID         string   `json:"stopId"`
	DepartureNano  int64    `json:"departureNano"` // nanoseconds from midnight of service day
	ServiceDay     string   `json:"serviceDay"`    // "YYYY-MM-DD"
}

// ScheduleForStop returns pre-filtered departure candidates for a stop code,
// doing all filtering (last stop, service active, time window) in-memory.
func (s *StaticStore) ScheduleForStop(code string, now time.Time, lookAhead time.Duration) []ScheduleCandidate {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stopIDs := s.stopCodes[code]
	if len(stopIDs) == 0 {
		return nil
	}

	today := truncateDay(now)
	yesterday := today.Add(-24 * time.Hour)

	stopIDSet := make(map[string]bool, len(stopIDs))
	for _, id := range stopIDs {
		stopIDSet[id] = true
	}

	type rawCandidate struct {
		dep        ScheduledDeparture
		stopID     string
		serviceDay time.Time
		adjusted   time.Time
	}

	var candidates []rawCandidate

	for _, stopID := range stopIDs {
		deps := s.stopIndex[stopID]
		for i := range deps {
			dep := &deps[i]

			// Skip trips where this stop is the final stop.
			trip, ok := s.tripIndex[dep.TripID]
			if !ok || len(trip.Stops) == 0 {
				continue
			}
			if stopIDSet[trip.Stops[len(trip.Stops)-1].StopID] {
				continue
			}

			for _, serviceDay := range []time.Time{today, yesterday} {
				svc, ok := s.services[dep.ServiceID]
				if !ok || !serviceActive(&svc, serviceDay) {
					continue
				}

				scheduled := serviceDay.Add(time.Duration(dep.DepartureTime))
				if scheduled.Before(now) || scheduled.After(now.Add(lookAhead)) {
					continue
				}

				candidates = append(candidates, rawCandidate{*dep, stopID, serviceDay, scheduled})
				break
			}
		}
	}

	// Sort by scheduled time.
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].adjusted.Before(candidates[j].adjusted)
	})

	// Deduplicate.
	seenTrip := make(map[string]bool)
	seenTimeLine := make(map[string]bool)
	const maxResults = 20
	result := make([]ScheduleCandidate, 0, maxResults)

	for i := range candidates {
		c := &candidates[i]
		tripNum := extractTripNum(c.dep.TripID)
		if seenTrip[tripNum] {
			continue
		}
		seenTrip[tripNum] = true

		timeLineKey := fmtTime(c.serviceDay.Add(time.Duration(c.dep.DepartureTime))) + "|" + c.dep.RouteID + "|" + c.stopID
		if seenTimeLine[timeLineKey] {
			continue
		}
		seenTimeLine[timeLineKey] = true

		route := s.routes[c.dep.RouteID]
		trip := s.tripIndex[c.dep.TripID]

		// Remaining stop names.
		var stops []string
		found := false
		for _, ts := range trip.Stops {
			if !found {
				if stopIDSet[ts.StopID] {
					found = true
				}
				continue
			}
			if name := s.stopNames[ts.StopID]; name != "" {
				stops = append(stops, name)
			}
		}

		// Platform from stop name.
		platform := extractPlat(s.stopNames[c.stopID])

		// Express detection.
		isExpress := false
		if len(trip.Stops) >= 2 {
			key := trip.RouteID + "|" + trip.Stops[0].StopID + "|" + trip.Stops[len(trip.Stops)-1].StopID
			if max := s.maxRouteStops[key]; max > 0 && len(trip.Stops) < max {
				isExpress = true
			}
		}

		result = append(result, ScheduleCandidate{
			TripID:         c.dep.TripID,
			TripNumber:     tripNum,
			RouteShortName: route.ShortName,
			RouteLongName:  route.LongName,
			RouteColor:     route.Color,
			RouteType:      route.Type,
			Headsign:       c.dep.Headsign,
			ScheduledTime:  fmtTime(c.serviceDay.Add(time.Duration(c.dep.DepartureTime))),
			Platform:       platform,
			Stops:          stops,
			IsExpress:      isExpress,
			StopID:         c.stopID,
			DepartureNano:  c.dep.DepartureTime,
			ServiceDay:     c.serviceDay.Format("2006-01-02"),
		})

		if len(result) >= maxResults {
			break
		}
	}

	return result
}

func truncateDay(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}

func fmtTime(t time.Time) string {
	return fmt.Sprintf("%02d:%02d", t.Hour(), t.Minute())
}

func extractTripNum(tripID string) string {
	if idx := strings.LastIndex(tripID, "-"); idx >= 0 && idx+1 < len(tripID) {
		return tripID[idx+1:]
	}
	return tripID
}

func extractPlat(stopName string) string {
	const prefix = "Platform "
	if idx := strings.LastIndex(stopName, prefix); idx >= 0 {
		return stopName[idx+len(prefix):]
	}
	return ""
}

// ArrivalTimeAtStop returns the scheduled arrival duration-from-midnight for a trip
// at the first matching stop from destStopIDs that appears AFTER any originStopIDs
// in the stop sequence.
func (s *StaticStore) ArrivalTimeAtStop(tripID string, destStopIDs []string, originStopIDs ...string) (time.Duration, bool) {
	s.mu.RLock()
	trip, ok := s.tripIndex[tripID]
	s.mu.RUnlock()
	if !ok {
		return 0, false
	}
	destSet := make(map[string]bool, len(destStopIDs))
	for _, id := range destStopIDs {
		destSet[id] = true
	}
	startIdx := 0
	if len(originStopIDs) > 0 {
		originSet := make(map[string]bool, len(originStopIDs))
		for _, id := range originStopIDs {
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
			return 0, false
		}
	}
	for i := startIdx; i < len(trip.Stops); i++ {
		if destSet[trip.Stops[i].StopID] {
			return time.Duration(trip.Stops[i].ArrivalTime), true
		}
	}
	return 0, false
}

// IsServiceActive returns true if the given service is operating on the given date.
func (s *StaticStore) IsServiceActive(serviceID string, date time.Time) bool {
	s.mu.RLock()
	svc, ok := s.services[serviceID]
	s.mu.RUnlock()
	if !ok {
		return false
	}
	return serviceActive(&svc, date)
}

func serviceActive(svc *gtfs.Service, date time.Time) bool {
	y, m, d := date.Date()

	for _, removed := range svc.RemovedDates {
		ry, rm, rd := removed.Date()
		if y == ry && m == rm && d == rd {
			return false
		}
	}
	for _, added := range svc.AddedDates {
		ay, am, ad := added.Date()
		if y == ay && m == am && d == ad {
			return true
		}
	}

	sy, sm, sd := svc.StartDate.Date()
	ey, em, ed := svc.EndDate.Date()
	startVal := sy*10000 + int(sm)*100 + sd
	endVal := ey*10000 + int(em)*100 + ed
	dateVal := y*10000 + int(m)*100 + d
	if dateVal < startVal || dateVal > endVal {
		return false
	}

	switch date.Weekday() {
	case time.Monday:
		return svc.Monday
	case time.Tuesday:
		return svc.Tuesday
	case time.Wednesday:
		return svc.Wednesday
	case time.Thursday:
		return svc.Thursday
	case time.Friday:
		return svc.Friday
	case time.Saturday:
		return svc.Saturday
	case time.Sunday:
		return svc.Sunday
	}
	return false
}
