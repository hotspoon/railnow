// Package app wires RailNow's HTTP routes to its services.
package app

import (
	"database/sql"

	"github.com/go-chi/chi/v5"
	"github.com/hotspoon/railnow/internal/handler"
	"github.com/hotspoon/railnow/internal/repository"
	"github.com/hotspoon/railnow/internal/service"
)

// NewHandler returns the dynamic RailNow application routes. Static files are
// served by the hosting platform in production and by cmd/server locally.
func NewHandler(db *sql.DB) *chi.Mux {
	s := service.New(repository.New(db))
	h := handler.New(s)
	r := chi.NewRouter()
	r.Get("/", h.Home)
	r.Get("/search", h.Search)
	r.Get("/schedule", h.Schedule)
	r.Get("/stations/destinations", h.Destinations)
	r.Get("/stations/destination-options", h.DestinationOptions)
	r.Get("/train/{id}", h.Train)
	r.Get("/saved", h.Saved)
	r.Post("/api/saved-routes/schedules", h.SavedRouteSchedules)
	return r
}
