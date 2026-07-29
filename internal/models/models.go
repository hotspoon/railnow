package models

type Station struct {
	ID               int64
	Code, Name, Line string
}
type Departure struct {
	TrainID                           int64
	Number, Route, Departure, Arrival string
	Duration                          int
	NextDay                           bool
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
	Stale                                                                  bool
}
type SearchPage struct {
	From, To     Station
	Departures   []Departure
	Transfers    []Itinerary
	ScheduleInfo ScheduleInfo
}
