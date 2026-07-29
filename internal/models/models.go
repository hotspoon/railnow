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
type Stop struct {
	Name, Arrival, Departure string
	Sequence                 int
}
type Favorite struct {
	ID, FromID, ToID int64
	Label, From, To  string
}
type ScheduleInfo struct{ SnapshotDate, DayType string }
type SearchPage struct {
	From, To     Station
	Departures   []Departure
	IsFavorite   bool
	ScheduleInfo ScheduleInfo
}
