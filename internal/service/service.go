package service

import (
	"context"
	"github.com/hotspoon/railnow/internal/models"
	"github.com/hotspoon/railnow/internal/repository"
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
func (s *Service) Home(ctx context.Context) ([]models.Favorite, []models.Favorite, models.ScheduleInfo, error) {
	f, e := s.repo.Favorites(ctx)
	if e != nil {
		return nil, nil, models.ScheduleInfo{}, e
	}
	r, e := s.repo.Recents(ctx)
	if e != nil {
		return nil, nil, models.ScheduleInfo{}, e
	}
	info, e := s.repo.ScheduleInfo(ctx)
	return f, r, info, e
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
	if len(d) == 0 {
		d, e = s.repo.Departures(ctx, from, to, "")
		if e != nil {
			return models.SearchPage{}, e
		}
		for i := range d {
			d[i].NextDay = true
		}
	}
	_ = s.repo.RecordSearch(ctx, from, to)
	fav, e := s.repo.IsFavorite(ctx, from, to)
	if e != nil {
		return models.SearchPage{}, e
	}
	info, e := s.repo.ScheduleInfo(ctx)
	return models.SearchPage{From: a, To: b, Departures: d, IsFavorite: fav, ScheduleInfo: info}, e
}
func (s *Service) Stops(ctx context.Context, id int64) ([]models.Stop, error) {
	return s.repo.Stops(ctx, id)
}
func (s *Service) ToggleFavorite(ctx context.Context, from, to int64) (bool, error) {
	return s.repo.ToggleFavorite(ctx, from, to)
}
