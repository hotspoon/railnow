package service

import (
	"context"
	"errors"
	"fmt"
	"github.com/hotspoon/railnow/internal/models"
	"github.com/hotspoon/railnow/internal/repository"
	"sort"
	"strings"
	"time"
)

var ErrInvalidSearchTime = errors.New("invalid search time")

type Service struct {
	repo *repository.Repository
	now  func() time.Time
}

func New(repo *repository.Repository) *Service {
	return NewWithClock(repo, time.Now)
}
func NewWithClock(repo *repository.Repository, clock func() time.Time) *Service {
	return &Service{repo: repo, now: clock}
}
func (s *Service) Stations(ctx context.Context) ([]models.Station, error) {
	return s.repo.Stations(ctx)
}
func (s *Service) Destinations(ctx context.Context, from int64) ([]models.Station, error) {
	return s.repo.Destinations(ctx, from)
}
func (s *Service) ScheduleInfo(ctx context.Context) (models.ScheduleInfo, error) {
	return s.repo.ScheduleInfo(ctx)
}
func (s *Service) Home(ctx context.Context) (models.ScheduleInfo, error) {
	return s.repo.ScheduleInfo(ctx)
}
func (s *Service) Search(ctx context.Context, from, to int64, selectedTime string) (models.SearchPage, error) {
	a, e := s.repo.Station(ctx, from)
	if e != nil {
		return models.SearchPage{}, e
	}
	b, e := s.repo.Station(ctx, to)
	if e != nil {
		return models.SearchPage{}, e
	}
	jakarta, _ := time.LoadLocation("Asia/Jakarta")
	current := s.now().In(jakarta)
	threshold, dayOffset, e := searchThreshold(current, selectedTime)
	if e != nil {
		return models.SearchPage{}, e
	}
	d, e := s.repo.Departures(ctx, from, to, threshold)
	if e != nil {
		return models.SearchPage{}, e
	}
	hasUpcomingDirect := len(d) > 0
	if !hasUpcomingDirect {
		d, e = s.repo.Departures(ctx, from, to, "")
		if e != nil {
			return models.SearchPage{}, e
		}
		dayOffset++
	}
	for i := range d {
		d[i].DayOffset = dayOffset
	}
	info, e := s.repo.ScheduleInfo(ctx)
	page := models.SearchPage{From: a, To: b, Departures: d, ScheduleInfo: info, SearchTime: selectedTime}
	if !hasUpcomingDirect {
		page.Transfers, e = s.Transfers(ctx, a, b, threshold, dayOffset-1)
	}
	return page, e
}

func searchThreshold(now time.Time, selected string) (string, int, error) {
	current := now.Format("15:04")
	if selected == "" {
		return current, 0, nil
	}
	if err := ValidateSearchTime(selected); err != nil {
		return "", 0, ErrInvalidSearchTime
	}
	offset := 0
	if selected < current {
		offset = 1
	}
	return selected, offset, nil
}

func ValidateSearchTime(selected string) error {
	if selected == "" {
		return nil
	}
	parsed, err := time.Parse("15:04", selected)
	if err != nil || parsed.Format("15:04") != selected {
		return ErrInvalidSearchTime
	}
	return nil
}
func (s *Service) Stops(ctx context.Context, id int64) ([]models.Stop, error) {
	return s.repo.Stops(ctx, id)
}
func (s *Service) ToggleFavorite(ctx context.Context, from, to int64) (bool, error) {
	return s.repo.ToggleFavorite(ctx, from, to)
}

// Transfers finds at most one connection through a supported interchange.
func (s *Service) Transfers(ctx context.Context, from, to models.Station, threshold string, dayOffset int) ([]models.Itinerary, error) {
	stations, err := s.Stations(ctx)
	if err != nil {
		return nil, err
	}
	hubs := map[string]bool{"manggarai": true, "tanah abang": true, "duri": true, "jatinegara": true}
	var out []models.Itinerary
	for _, hub := range stations {
		if !hubs[strings.ToLower(hub.Name)] || hub.ID == from.ID || hub.ID == to.ID {
			continue
		}
		first, err := s.repo.Departures(ctx, from.ID, hub.ID, threshold)
		if err != nil {
			return nil, err
		}
		second, err := s.repo.Departures(ctx, hub.ID, to.ID, "")
		if err != nil {
			return nil, err
		}
		for _, a := range first {
			for _, b := range second {
				if a.TrainID == b.TrainID {
					continue
				}
				wait := minutes(a.Arrival, b.Departure)
				if wait < 5 {
					continue
				}
				a.DayOffset = dayOffset
				b.DayOffset = dayOffset
				if b.Departure < a.Arrival {
					b.DayOffset++
				}
				out = append(out, models.Itinerary{First: a, Second: b, Transfer: hub, WaitMinutes: wait, TotalMinutes: a.Duration + wait + b.Duration})
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].TotalMinutes < out[j].TotalMinutes })
	if len(out) > 3 {
		out = out[:3]
	}
	return out, nil
}

func (s *Service) SavedSchedules(ctx context.Context, routes []models.SavedRouteInput) []models.SavedRouteSchedule {
	out := make([]models.SavedRouteSchedule, 0, len(routes))
	stations, err := s.Stations(ctx)
	if err != nil {
		for _, route := range routes {
			out = append(out, models.SavedRouteSchedule{From: route.From, To: route.To, Status: "error"})
		}
		return out
	}
	stationsByID := make(map[int64]models.Station, len(stations))
	for _, station := range stations {
		stationsByID[station.ID] = station
	}
	jakarta, _ := time.LoadLocation("Asia/Jakarta")
	threshold := s.now().In(jakarta).Format("15:04")
	for _, route := range routes {
		item := models.SavedRouteSchedule{From: route.From, To: route.To, Status: "invalid_route"}
		from, fromExists := stationsByID[route.From]
		to, toExists := stationsByID[route.To]
		if route.From <= 0 || route.To <= 0 || route.From == route.To || !fromExists || !toExists {
			out = append(out, item)
			continue
		}
		item.FromName = from.Name
		item.ToName = to.Name
		departures, err := s.repo.Departures(ctx, route.From, route.To, threshold)
		if err != nil {
			item.Status = "error"
			out = append(out, item)
			continue
		}
		dayOffset := 0
		if len(departures) == 0 {
			departures, err = s.repo.Departures(ctx, route.From, route.To, "")
			dayOffset = 1
		}
		if err != nil {
			item.Status = "error"
		} else if len(departures) == 0 {
			item.Status = "no_service"
		} else {
			item.Status = "ok"
			next := departures[0]
			next.DayOffset = dayOffset
			item.Next = &next
		}
		out = append(out, item)
	}
	return out
}

func minutes(start, end string) int {
	base := "2006-01-02 "
	parse := func(value string) (time.Time, error) {
		for _, layout := range []string{"2006-01-02 15:04:05", "2006-01-02 15:04"} {
			if parsed, err := time.Parse(layout, base+value); err == nil {
				return parsed, nil
			}
		}
		return time.Time{}, fmt.Errorf("invalid timetable value %q", value)
	}
	a, err := parse(start)
	if err != nil {
		return 0
	}
	b, err := parse(end)
	if err != nil {
		return 0
	}
	if b.Before(a) {
		b = b.Add(24 * time.Hour)
	}
	return int(b.Sub(a).Minutes())
}
