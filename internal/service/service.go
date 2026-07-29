package service

import (
	"context"
	"github.com/hotspoon/railnow/internal/models"
	"github.com/hotspoon/railnow/internal/repository"
)

type Service struct{ repo *repository.Repository }

func New(repo *repository.Repository) *Service { return &Service{repo: repo} }
func (s *Service) Stations(ctx context.Context) ([]models.Station, error) {
	return s.repo.Stations(ctx)
}
func (s *Service) Home(ctx context.Context) ([]models.Favorite, []models.Favorite, error) {
	f, e := s.repo.Favorites(ctx)
	if e != nil {
		return nil, nil, e
	}
	r, e := s.repo.Recents(ctx)
	return f, r, e
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
	d, e := s.repo.Departures(ctx, from, to)
	if e != nil {
		return models.SearchPage{}, e
	}
	_ = s.repo.RecordSearch(ctx, from, to)
	fav, e := s.repo.IsFavorite(ctx, from, to)
	return models.SearchPage{From: a, To: b, Departures: d, IsFavorite: fav}, e
}
func (s *Service) Stops(ctx context.Context, id int64) ([]models.Stop, error) {
	return s.repo.Stops(ctx, id)
}
func (s *Service) ToggleFavorite(ctx context.Context, from, to int64) (bool, error) {
	return s.repo.ToggleFavorite(ctx, from, to)
}
