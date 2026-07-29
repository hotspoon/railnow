package service

import (
	"context"
	"fmt"
	"github.com/hotspoon/railnow/internal/models"
	"github.com/hotspoon/railnow/internal/repository"
	"sort"
	"strings"
	"time"
)

type Service struct{ repo *repository.Repository }

func New(repo *repository.Repository) *Service { return &Service{repo: repo} }
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
func (s *Service) Search(ctx context.Context, from, to int64) (models.SearchPage, error) {
	a, e := s.repo.Station(ctx, from)
	if e != nil {
		return models.SearchPage{}, e
	}
	b, e := s.repo.Station(ctx, to)
	if e != nil {
		return models.SearchPage{}, e
	}
	jakarta, _ := time.LoadLocation("Asia/Jakarta")
	now := time.Now().In(jakarta).Format("15:04")
	d, e := s.repo.Departures(ctx, from, to, now)
	if e != nil {
		return models.SearchPage{}, e
	}
	hasUpcomingDirect := len(d) > 0
	if !hasUpcomingDirect {
		d, e = s.repo.Departures(ctx, from, to, "")
		if e != nil {
			return models.SearchPage{}, e
		}
		for i := range d {
			d[i].NextDay = true
		}
	}
	info, e := s.repo.ScheduleInfo(ctx)
	page := models.SearchPage{From: a, To: b, Departures: d, ScheduleInfo: info}
	if !hasUpcomingDirect {
		page.Transfers, e = s.Transfers(ctx, a, b, now)
	}
	return page, e
}
func (s *Service) Stops(ctx context.Context, id int64) ([]models.Stop, error) {
	return s.repo.Stops(ctx, id)
}
func (s *Service) ToggleFavorite(ctx context.Context, from, to int64) (bool, error) {
	return s.repo.ToggleFavorite(ctx, from, to)
}

// Transfers finds at most one connection through a supported interchange.
func (s *Service) Transfers(ctx context.Context, from, to models.Station, now string) ([]models.Itinerary, error) {
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
		first, err := s.repo.Departures(ctx, from.ID, hub.ID, now)
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
