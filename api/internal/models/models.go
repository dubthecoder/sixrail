package models

type Stop struct {
	ID       string  `json:"id"`
	Code     string  `json:"code"`
	Name     string  `json:"name"`
	Lat      float64 `json:"lat"`
	Lon      float64 `json:"lon"`
	ParentID string  `json:"parentId,omitempty"`
}

type Route struct {
	ID        string `json:"id"`
	ShortName string `json:"shortName"`
	LongName  string `json:"longName"`
	Color     string `json:"color"`
	TextColor string `json:"textColor"`
	Type      int    `json:"type"`
}

type VehiclePosition struct {
	VehicleID     string  `json:"vehicleId"`
	TripID        string  `json:"tripId"`
	RouteID       string  `json:"routeId"`
	RouteName     string  `json:"routeName"`
	RouteColor    string  `json:"routeColor"`
	RouteType     int     `json:"routeType"`
	Lat           float64 `json:"lat"`
	Lon           float64 `json:"lon"`
	Bearing       float32 `json:"bearing,omitempty"`
	Speed         float32 `json:"speed,omitempty"`
	Timestamp     int64   `json:"timestamp"`
	CurrentStatus string  `json:"currentStatus,omitempty"`
	NextStopID    string  `json:"nextStopId,omitempty"`
}

type Departure struct {
	Line          string   `json:"line"`
	Destination   string   `json:"destination"`
	ScheduledTime string   `json:"scheduledTime"` // "HH:MM" local time
	ArrivalTime   string   `json:"arrivalTime,omitempty"` // "HH:MM" arrival at destination stop
	Status        string   `json:"status"`        // "On Time", "Delayed +Xm", "Cancelled"
	Platform      string   `json:"platform,omitempty"`
	RouteColor    string   `json:"routeColor,omitempty"`
	DelayMinutes  int      `json:"delayMinutes,omitempty"`
	Stops         []string `json:"stops,omitempty"`
}

type RouteShape struct {
	RouteID   string      `json:"routeId"`
	RouteName string      `json:"routeName"`
	Color     string      `json:"color"`
	Points    [][2]float64 `json:"points"` // [lon, lat] pairs
}

type TripDetail struct {
	TripID        string           `json:"tripId"`
	VehicleID     string           `json:"vehicleId"`
	RouteName     string           `json:"routeName"`
	RouteColor    string           `json:"routeColor"`
	Origin        string           `json:"origin"`
	Destination   string           `json:"destination"`
	ScheduleStart string           `json:"scheduleStart"` // "HH:MM"
	ScheduleEnd   string           `json:"scheduleEnd"`   // "HH:MM"
	Status        string           `json:"status"`        // "On Time", "Delayed +3m", "Cancelled"
	DelayMinutes  int              `json:"delayMinutes"`
	CurrentStop   string           `json:"currentStop,omitempty"`
	UpcomingStops []UpcomingStop   `json:"upcomingStops"`
}

type UpcomingStop struct {
	Name         string `json:"name"`
	Platform     string `json:"platform,omitempty"`
	Time         string `json:"time"`         // "4:27 p.m."
	DelayMinutes int    `json:"delayMinutes"`
}

// NextServiceLine is a single real-time next-service entry from Metrolinx NextService API.
type NextServiceLine struct {
	LineCode      string `json:"lineCode"`
	LineName      string `json:"lineName"`
	Direction     string `json:"direction"`
	ScheduledTime string `json:"scheduledTime"` // "HH:MM"
	ComputedTime  string `json:"computedTime"`  // "HH:MM" real-time adjusted
	Platform      string `json:"platform,omitempty"`
	ActualPlatform string `json:"actualPlatform,omitempty"`
	TripNumber    string `json:"tripNumber"`
	Status        string `json:"status"` // "On Time", "Delayed", "Moving"
}

// UnionDeparture is a single departure from the Union Station departures board.
type UnionDeparture struct {
	TripNumber string   `json:"tripNumber"`
	Service    string   `json:"service"`
	Platform   string   `json:"platform"`
	Time       string   `json:"time"` // "HH:MM"
	Info       string   `json:"info"` // "Proceed", "Wait", etc.
	Stops      []string `json:"stops"`
}

type Alert struct {
	ID          string   `json:"id"`
	Effect      string   `json:"effect"`
	Headline    string   `json:"headline"`
	Description string   `json:"description"`
	URL         string   `json:"url,omitempty"`
	RouteIDs    []string `json:"routeIds,omitempty"`
	RouteNames  []string `json:"routeNames,omitempty"`
	StartTime   int64    `json:"startTime,omitempty"`
	EndTime     int64    `json:"endTime,omitempty"`
}
