package gtfs

import (
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/jamespfennell/gtfs"

	"github.com/teclara/sixrail/api/internal/models"
)

// ScheduledDeparture is a single stop-time entry in the schedule index.
type ScheduledDeparture struct {
	TripID        string
	RouteID       string
	ServiceID     string
	Headsign      string
	DepartureTime time.Duration // duration from midnight (local time)
}

// TripStop is one stop in a trip's ordered sequence, used for position simulation.
type TripStop struct {
	StopID        string
	Lat           float64
	Lon           float64
	ArrivalTime   time.Duration // duration from midnight of service day
	DepartureTime time.Duration
}

// SimTrip holds a trip's identity and full stop sequence for position simulation.
type SimTrip struct {
	TripID    string
	RouteID   string
	ServiceID string
	Stops     []TripStop
}

type StaticStore struct {
	mu        sync.RWMutex
	stops     []models.Stop
	routes    map[string]models.Route
	stopIndex map[string][]ScheduledDeparture // stopID → sorted departures
	stopCodes map[string][]string             // stopCode → []stopID (parent + children)
	services  map[string]*gtfs.Service        // serviceID → service
	tripIndex map[string]SimTrip              // tripID → SimTrip
}

func NewStaticStore(zipData []byte) (*StaticStore, error) {
	s := &StaticStore{}
	if err := s.load(zipData); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *StaticStore) load(zipData []byte) error {
	static, err := gtfs.ParseStatic(zipData, gtfs.ParseStaticOptions{})
	if err != nil {
		return fmt.Errorf("parsing GTFS static: %w", err)
	}

	// --- Stops ---
	stops := make([]models.Stop, 0, len(static.Stops))
	for i := range static.Stops {
		gs := &static.Stops[i]
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
	services := make(map[string]*gtfs.Service, len(static.Services))
	for i := range static.Services {
		svc := &static.Services[i]
		services[svc.Id] = svc
	}

	// --- Stop code → stop IDs mapping ---
	// A stop code can correspond to a parent station plus child platforms.
	stopCodes := make(map[string][]string)
	for i := range static.Stops {
		gs := &static.Stops[i]
		if gs.Code == "" {
			continue
		}
		stopCodes[gs.Code] = append(stopCodes[gs.Code], gs.Id)
		// Also index child stops under the parent's code.
		if gs.Parent != nil && gs.Parent.Code != "" {
			stopCodes[gs.Parent.Code] = appendUnique(stopCodes[gs.Parent.Code], gs.Id)
		}
	}

	// --- Schedule index: stopID → []ScheduledDeparture ---
	stopIndex := make(map[string][]ScheduledDeparture)
	for i := range static.Trips {
		trip := &static.Trips[i]
		if trip.Route == nil || trip.Service == nil {
			continue
		}
		// Use trip headsign; fall back to route long name.
		headsign := trip.Headsign
		if headsign == "" {
			headsign = trip.Route.LongName
		}
		for j := range trip.StopTimes {
			st := &trip.StopTimes[j]
			if st.Stop == nil {
				continue
			}
			dep := ScheduledDeparture{
				TripID:        trip.ID,
				RouteID:       trip.Route.Id,
				ServiceID:     trip.Service.Id,
				Headsign:      headsign,
				DepartureTime: st.DepartureTime,
			}
			stopID := st.Stop.Id
			stopIndex[stopID] = append(stopIndex[stopID], dep)
		}
	}

	// --- Trip index for position simulation ---
	tripIndex := make(map[string]SimTrip, len(static.Trips))
	for i := range static.Trips {
		trip := &static.Trips[i]
		if trip.Route == nil || trip.Service == nil {
			continue
		}
		stops := make([]TripStop, 0, len(trip.StopTimes))
		for j := range trip.StopTimes {
			st := &trip.StopTimes[j]
			if st.Stop == nil || st.Stop.Latitude == nil || st.Stop.Longitude == nil {
				continue
			}
			stops = append(stops, TripStop{
				StopID:        st.Stop.Id,
				Lat:           *st.Stop.Latitude,
				Lon:           *st.Stop.Longitude,
				ArrivalTime:   st.ArrivalTime,
				DepartureTime: st.DepartureTime,
			})
		}
		if len(stops) < 2 {
			continue // need at least 2 stops to interpolate
		}
		tripIndex[trip.ID] = SimTrip{
			TripID:    trip.ID,
			RouteID:   trip.Route.Id,
			ServiceID: trip.Service.Id,
			Stops:     stops,
		}
	}

	s.mu.Lock()
	s.stops = stops
	s.routes = routes
	s.stopIndex = stopIndex
	s.stopCodes = stopCodes
	s.services = services
	s.tripIndex = tripIndex
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

// IsServiceActive returns true if the given service is operating on the given date.
func (s *StaticStore) IsServiceActive(serviceID string, date time.Time) bool {
	s.mu.RLock()
	svc, ok := s.services[serviceID]
	s.mu.RUnlock()
	if !ok {
		return false
	}
	return serviceActive(svc, date)
}

// ActiveSimTrips returns all trips whose service is active on the given date.
// Used by the position simulator to find trips currently running.
func (s *StaticStore) ActiveSimTrips(now time.Time) []SimTrip {
	loc, _ := time.LoadLocation("America/Toronto")
	nowLocal := now.In(loc)
	today := truncateToDay(nowLocal)

	s.mu.RLock()
	defer s.mu.RUnlock()

	out := make([]SimTrip, 0, 256)
	for _, trip := range s.tripIndex {
		svc, ok := s.services[trip.ServiceID]
		if !ok {
			continue
		}
		if serviceActive(svc, today) {
			out = append(out, trip)
		}
	}
	return out
}

func serviceActive(svc *gtfs.Service, date time.Time) bool {
	// Compare calendar dates only (year, month, day) to avoid timezone instant mismatches.
	// GTFS dates from the parser may be in UTC while our date is in America/Toronto.
	y, m, d := date.Date()

	// Check added/removed exception dates first.
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

	// Check date range using calendar dates, not instants.
	sy, sm, sd := svc.StartDate.Date()
	ey, em, ed := svc.EndDate.Date()
	startVal := sy*10000 + int(sm)*100 + sd
	endVal := ey*10000 + int(em)*100 + ed
	dateVal := y*10000 + int(m)*100 + d
	if dateVal < startVal || dateVal > endVal {
		return false
	}

	// Check weekday.
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
