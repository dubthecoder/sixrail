package gtfsrt

import (
	"time"

	"github.com/teclara/railsix/shared/models"
)

// RouteLookup is satisfied by any store that can resolve route IDs to Route structs.
type RouteLookup interface {
	GetRoute(id string) (models.Route, bool)
}

// --- JSON structures matching Metrolinx GTFS-RT JSON format ---

type gtfsRTFeed struct {
	Entity []gtfsRTEntity `json:"entity"`
}

type gtfsRTEntity struct {
	ID         string         `json:"id"`
	Alert      *gtfsRTAlert   `json:"alert"`
	TripUpdate *gtfsRTTripUpd `json:"trip_update"`
	Vehicle    *gtfsRTVehicle `json:"vehicle"`
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
	ActivePeriod   []gtfsRTActivePeriod `json:"active_period"`
	InformedEntity []gtfsRTInformedEnt  `json:"informed_entity"`
	Effect         string               `json:"effect"`
	URL            *gtfsRTTranslatedStr `json:"url"`
	HeaderText     *gtfsRTTranslatedStr `json:"header_text"`
	DescriptionTxt *gtfsRTTranslatedStr `json:"description_text"`
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
	StopID               string       `json:"stop_id"`
	Arrival              *gtfsRTDelay `json:"arrival"`
	Departure            *gtfsRTDelay `json:"departure"`
	ScheduleRelationship string       `json:"schedule_relationship"`
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
