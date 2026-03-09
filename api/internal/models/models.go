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

type Departure struct {
	Line          string   `json:"line"`
	LineName      string   `json:"lineName,omitempty"`
	Destination   string   `json:"destination"`
	ScheduledTime string   `json:"scheduledTime"`         // "HH:MM" local time
	ActualTime    string   `json:"actualTime,omitempty"`  // "HH:MM" real-time adjusted departure
	ArrivalTime   string   `json:"arrivalTime,omitempty"` // "HH:MM" arrival at destination stop
	Status        string   `json:"status"`                // "On Time", "Delayed +Xm", "Cancelled"
	Platform      string   `json:"platform,omitempty"`
	RouteColor    string   `json:"routeColor,omitempty"`
	DelayMinutes  int      `json:"delayMinutes,omitempty"`
	Stops         []string `json:"stops,omitempty"`
	Cars          string   `json:"cars,omitempty"` // number of coaches
	IsInMotion    bool     `json:"isInMotion,omitempty"`
	IsCancelled   bool     `json:"isCancelled,omitempty"`
	IsExpress     bool     `json:"isExpress,omitempty"`
	RouteType     int      `json:"routeType"`
}

// NextServiceLine is a single real-time next-service entry from Metrolinx NextService API.
type NextServiceLine struct {
	LineCode       string `json:"lineCode"`
	LineName       string `json:"lineName"`
	Direction      string `json:"direction"`
	ScheduledTime  string `json:"scheduledTime"` // "HH:MM"
	ComputedTime   string `json:"computedTime"`  // "HH:MM" real-time adjusted
	Platform       string `json:"platform,omitempty"`
	ActualPlatform string `json:"actualPlatform,omitempty"`
	TripNumber     string `json:"tripNumber"`
	Status         string `json:"status"` // "On Time", "Delayed", "Moving"
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

// NetworkLine represents the count of active trains on a GO Transit line.
type NetworkLine struct {
	LineCode    string `json:"lineCode"`
	LineName    string `json:"lineName"`
	ActiveTrips int    `json:"activeTrips"`
}

// FareInfo represents a single fare option between two stations.
type FareInfo struct {
	Category   string  `json:"category"`   // e.g. "Adult", "Senior/Youth"
	TicketType string  `json:"ticketType"` // e.g. "Single Ride", "Day Pass"
	FareType   string  `json:"fareType"`   // e.g. "PRESTO", "Cash"
	Amount     float64 `json:"amount"`
}

// ServiceGlanceEntry holds cached data from the ServiceataGlance/Trains/All endpoint.
type ServiceGlanceEntry struct {
	TripNumber string
	LineCode   string
	LineName   string // Display field
	Cars       string
	Lat        float64
	Lon        float64
	IsInMotion bool
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
