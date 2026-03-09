package gtfs

import (
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/jamespfennell/gtfs"

	"github.com/teclara/railsix/api/internal/models"
)

// ScheduledDeparture is a single stop-time entry in the schedule index.
type ScheduledDeparture struct {
	TripID        string
	RouteID       string
	ServiceID     string
	Headsign      string
	DepartureTime time.Duration // duration from midnight (local time)
}

// TripStop is one stop in a trip's ordered sequence.
type TripStop struct {
	StopID        string
	ArrivalTime   time.Duration // duration from midnight of service day
	DepartureTime time.Duration
}

// TripInfo holds a trip's identity and full stop sequence for departure enrichment.
type TripInfo struct {
	TripID    string
	RouteID   string
	ServiceID string
	Stops     []TripStop
}

type StaticStore struct {
	mu        sync.RWMutex
	loaded    bool // true once GTFS data has been successfully loaded at least once
	stops     []models.Stop
	stopNames map[string]string // stopID → name
	routes    map[string]models.Route
	stopIndex map[string][]ScheduledDeparture // stopID → sorted departures
	stopCodes map[string][]string             // stopCode → []stopID (parent + children)
	services  map[string]gtfs.Service         // serviceID → service
	tripIndex     map[string]TripInfo             // tripID → TripInfo
	maxRouteStops map[string]int                  // routeID|firstStop|lastStop → max stop count
}

// NewStaticStore creates a StaticStore and loads the given GTFS ZIP data.
// Returns a ready store on success.
func NewStaticStore(zipData []byte) (*StaticStore, error) {
	s := &StaticStore{}
	if err := s.load(zipData); err != nil {
		return nil, err
	}
	return s, nil
}

// NewEmptyStaticStore creates a StaticStore with no data loaded.
// The store starts in a not-ready state; call Refresh to load data.
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
	// Copy by value to avoid pinning the entire parsed gtfs.Static in memory.
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
		tripStops := make([]TripStop, 0, len(trip.StopTimes))
		for j := range trip.StopTimes {
			st := &trip.StopTimes[j]
			if st.Stop == nil {
				continue
			}
			stopIndex[st.Stop.Id] = append(stopIndex[st.Stop.Id], ScheduledDeparture{
				TripID:        trip.ID,
				RouteID:       trip.Route.Id,
				ServiceID:     trip.Service.Id,
				Headsign:      headsign,
				DepartureTime: st.DepartureTime,
			})
			tripStops = append(tripStops, TripStop{
				StopID:        st.Stop.Id,
				ArrivalTime:   st.ArrivalTime,
				DepartureTime: st.DepartureTime,
			})
		}
		if len(tripStops) < 2 {
			continue
		}
		tripIndex[trip.ID] = TripInfo{
			TripID:    trip.ID,
			RouteID:   trip.Route.Id,
			ServiceID: trip.Service.Id,
			Stops:     tripStops,
		}
	}

	// --- Max stop count per route+endpoint pair (for express detection) ---
	// Group by route + first/last stop so short-turn trips aren't falsely tagged express.
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

func (s *StaticStore) Refresh(zipData []byte) error {
	return s.load(zipData)
}

func (s *StaticStore) AllStops() []models.Stop {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]models.Stop, len(s.stops))
	copy(out, s.stops)
	return out
}

func (s *StaticStore) GetStopName(id string) string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.stopNames[id]
}

func (s *StaticStore) GetTrip(tripID string) (TripInfo, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	t, ok := s.tripIndex[tripID]
	return t, ok
}

// RemainingStopNames returns the names of stops after the given stopIDs in a trip.
func (s *StaticStore) RemainingStopNames(tripID string, departureStopIDs []string) []string {
	trip, ok := s.GetTrip(tripID)
	if !ok {
		return nil
	}
	depSet := make(map[string]bool, len(departureStopIDs))
	for _, id := range departureStopIDs {
		depSet[id] = true
	}
	found := false
	s.mu.RLock()
	defer s.mu.RUnlock()
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
// Trips where the queried stop is the last stop are arrivals, not departures.
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

// ArrivalTimeAtStop returns the scheduled arrival duration-from-midnight for a trip
// at the first matching stop from destStopIDs that appears AFTER any originStopIDs
// in the stop sequence. This ensures we only match trips traveling from origin → destination.
// Returns 0, false if the destination is not found after the origin.
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
	// If origin stop IDs are provided, find the origin index first and only
	// match destinations that come after it in the stop sequence.
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
			return trip.Stops[i].ArrivalTime, true
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

func truncateToDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}
