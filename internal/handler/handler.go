package handler

import (
	"errors"
	"github.com/a-h/templ"
	"github.com/go-chi/chi/v5"
	"github.com/hotspoon/railnow/internal/models"
	"github.com/hotspoon/railnow/internal/service"
	"github.com/hotspoon/railnow/templates/components"
	"github.com/hotspoon/railnow/templates/pages"
	"net/http"
	"strconv"
)

type Handler struct{ service *service.Service }

func New(s *service.Service) *Handler { return &Handler{service: s} }
func render(w http.ResponseWriter, r *http.Request, c templ.Component) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := c.Render(r.Context(), w); err != nil {
		http.Error(w, "Unable to render page", 500)
	}
}
func (h *Handler) Home(w http.ResponseWriter, r *http.Request) {
	stations, e := h.service.Stations(r.Context())
	if e != nil {
		http.Error(w, "Could not load stations", 500)
		return
	}
	fromID := stationIDByCode(stations, "MRI")
	toID := stationIDByCode(stations, "BOO")
	info, e := h.service.Home(r.Context())
	if e != nil {
		http.Error(w, "Could not load routes", 500)
		return
	}
	render(w, r, pages.Home(stations, fromID, toID, info))
}
func stationIDByCode(stations []models.Station, code string) int64 {
	for _, station := range stations {
		if station.Code == code {
			return station.ID
		}
	}
	return 0
}
func ids(r *http.Request) (int64, int64, error) {
	from, e := strconv.ParseInt(r.FormValue("from"), 10, 64)
	if e != nil {
		return 0, 0, e
	}
	to, e := strconv.ParseInt(r.FormValue("to"), 10, 64)
	if e != nil || from == to {
		return 0, 0, errors.New("invalid route")
	}
	return from, to, nil
}
func (h *Handler) Search(w http.ResponseWriter, r *http.Request) {
	from, to, e := ids(r)
	if e != nil {
		http.Error(w, "Choose two different stations", 400)
		return
	}
	p, e := h.service.Search(r.Context(), from, to)
	if e != nil {
		http.Error(w, "Could not find schedule", 500)
		return
	}
	if r.Header.Get("HX-Request") == "true" {
		render(w, r, pages.SearchResult(p))
		return
	}
	render(w, r, pages.SearchPage(p))
}
func (h *Handler) Schedule(w http.ResponseWriter, r *http.Request) {
	from, to, e := ids(r)
	if e != nil {
		http.Error(w, "Invalid route", 400)
		return
	}
	p, e := h.service.Search(r.Context(), from, to)
	if e != nil {
		http.Error(w, "Could not refresh schedule", 500)
		return
	}
	render(w, r, pages.SearchResult(p))
}
func (h *Handler) Destinations(w http.ResponseWriter, r *http.Request) {
	from, e := strconv.ParseInt(r.URL.Query().Get("from"), 10, 64)
	if e != nil {
		http.Error(w, "Invalid origin", http.StatusBadRequest)
		return
	}
	destinations, e := h.service.Destinations(r.Context(), from)
	if e != nil {
		http.Error(w, "Could not load destinations", http.StatusInternalServerError)
		return
	}
	render(w, r, components.DestinationSelect(destinations, 0))
}
func (h *Handler) Train(w http.ResponseWriter, r *http.Request) {
	id, e := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if e != nil {
		http.NotFound(w, r)
		return
	}
	stops, e := h.service.Stops(r.Context(), id)
	if e != nil || len(stops) == 0 {
		http.NotFound(w, r)
		return
	}
	info, e := h.service.ScheduleInfo(r.Context())
	if e != nil {
		http.Error(w, "Could not load schedule metadata", 500)
		return
	}
	backURL := "/"
	if from, to, err := ids(r); err == nil {
		backURL = "/search?from=" + strconv.FormatInt(from, 10) + "&to=" + strconv.FormatInt(to, 10)
	}
	render(w, r, pages.TrainDetail(stops, info, backURL))
}
func (h *Handler) Saved(w http.ResponseWriter, r *http.Request) { render(w, r, pages.Saved()) }
