package models

type Station struct {
	ID               int64
	Code, Name, Line string
}
type Departure struct {
	TrainID   int64  `json:"train_id"`
	Number    string `json:"number"`
	Route     string `json:"route"`
	Departure string `json:"departure"`
	Arrival   string `json:"arrival"`
	Duration  int    `json:"duration_minutes"`
	DayOffset int    `json:"day_offset"`
}
type Itinerary struct {
	First, Second             Departure
	Transfer                  Station
	WaitMinutes, TotalMinutes int
}
type Stop struct {
	Name, Arrival, Departure string
	Sequence                 int
}
type Favorite struct {
	ID, FromID, ToID int64
	Label, From, To  string
}
type ScheduleInfo struct {
	SnapshotDate, DayType, EffectiveDate, SourceName, SourceURL, FetchedAt string
	UpdatedLabel, Status                                                   string
	Stale                                                                  bool
}
type SearchPage struct {
	From, To     Station
	Departures   []Departure
	Transfers    []Itinerary
	ScheduleInfo ScheduleInfo
	SearchTime   string
}

type SavedRouteInput struct {
	From int64 `json:"from"`
	To   int64 `json:"to"`
}

type SavedRouteSchedule struct {
	From     int64      `json:"from"`
	To       int64      `json:"to"`
	FromName string     `json:"from_name,omitempty"`
	ToName   string     `json:"to_name,omitempty"`
	Status   string     `json:"status"`
	Next     *Departure `json:"next,omitempty"`
}
