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
	VehicleID  string  `json:"vehicleId"`
	TripID     string  `json:"tripId"`
	RouteID    string  `json:"routeId"`
	RouteName  string  `json:"routeName"`
	RouteColor string  `json:"routeColor"`
	RouteType  int     `json:"routeType"`
	Lat        float64 `json:"lat"`
	Lon        float64 `json:"lon"`
	Bearing    float32 `json:"bearing,omitempty"`
	Speed      float32 `json:"speed,omitempty"`
	Timestamp  int64   `json:"timestamp"`
}

type Departure struct {
	Line          string `json:"line"`
	Destination   string `json:"destination"`
	ScheduledTime string `json:"scheduledTime"` // "HH:MM" local time
	Status        string `json:"status"`        // "On Time", "Delayed +Xm", "Cancelled"
	Platform      string `json:"platform,omitempty"`
	RouteColor    string `json:"routeColor,omitempty"`
	DelayMinutes  int    `json:"delayMinutes,omitempty"`
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
