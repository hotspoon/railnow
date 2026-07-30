package handler

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/hotspoon/railnow/internal/repository"
	"github.com/hotspoon/railnow/internal/service"
	_ "modernc.org/sqlite"
)

func TestSavedRouteSchedulesValidatesBatchLimit(t *testing.T) {
	handler := savedHandler(t)
	routes := make([]map[string]int, 11)
	for i := range routes {
		routes[i] = map[string]int{"from": 1, "to": 2}
	}
	body, _ := json.Marshal(map[string]any{"routes": routes})
	request := httptest.NewRequest(http.MethodPost, "/api/saved-routes/schedules", bytes.NewReader(body))
	response := httptest.NewRecorder()

	handler.SavedRouteSchedules(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", response.Code)
	}
}

func TestSavedRouteSchedulesReturnsPartialResultsWithoutCaching(t *testing.T) {
	handler := savedHandler(t)
	body := []byte(`{"routes":[{"from":1,"to":2},{"from":1,"to":3},{"from":1,"to":1}]}`)
	request := httptest.NewRequest(http.MethodPost, "/api/saved-routes/schedules", bytes.NewReader(body))
	response := httptest.NewRecorder()

	handler.SavedRouteSchedules(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", response.Code, response.Body.String())
	}
	if response.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("Cache-Control = %q", response.Header().Get("Cache-Control"))
	}
	var payload struct {
		DataState string `json:"data_status"`
		Routes    []struct {
			Status string `json:"status"`
		} `json:"routes"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.DataState != "available" {
		t.Errorf("data status = %q", payload.DataState)
	}
	want := []string{"ok", "no_service", "invalid_route"}
	for i := range want {
		if payload.Routes[i].Status != want[i] {
			t.Errorf("route %d = %q, want %q", i, payload.Routes[i].Status, want[i])
		}
	}
}

func TestSearchRejectsInvalidTime(t *testing.T) {
	handler := savedHandler(t)
	request := httptest.NewRequest(http.MethodGet, "/search?from=1&to=2&time=25:90", nil)
	response := httptest.NewRecorder()

	handler.Search(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", response.Code)
	}
}

func savedHandler(t *testing.T) *Handler {
	t.Helper()
	db, err := sql.Open("sqlite", "file:"+t.Name()+"?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	_, err = db.Exec(`
		CREATE TABLE stations (id INTEGER PRIMARY KEY, code TEXT, name TEXT, line TEXT);
		CREATE TABLE trains (id INTEGER PRIMARY KEY, train_number TEXT, route_name TEXT);
		CREATE TABLE schedules (train_id INTEGER, station_id INTEGER, arrival TEXT, departure TEXT, sequence INTEGER);
		CREATE TABLE schedule_metadata (key TEXT PRIMARY KEY, value TEXT);
		INSERT INTO stations VALUES
			(1, 'AAA', 'Alpha', 'Blue'),
			(2, 'BBB', 'Beta', 'Blue'),
			(3, 'CCC', 'Gamma', 'Blue');
		INSERT INTO trains VALUES
			(1, '1001', 'Alpha — Beta'),
			(2, '2001', 'Gamma local');
		INSERT INTO schedules VALUES
			(1, 1, '08:00', '08:00', 1),
			(1, 2, '08:30', '08:30', 2),
			(2, 3, '09:00', '09:00', 1);
		INSERT INTO schedule_metadata VALUES ('fetched_at', ?);`,
		time.Now().UTC().Format(time.RFC3339),
	)
	if err != nil {
		t.Fatal(err)
	}
	svc := service.NewWithClock(repository.New(db), func() time.Time {
		return time.Date(2026, 7, 30, 7, 0, 0, 0, time.FixedZone("WIB", 7*60*60))
	})
	if _, err := svc.ScheduleInfo(context.Background()); err != nil {
		t.Fatal(err)
	}
	return New(svc)
}
