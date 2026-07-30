package service

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/hotspoon/railnow/internal/models"
	"github.com/hotspoon/railnow/internal/repository"
	_ "modernc.org/sqlite"
)

func TestMinutesHandlesOvernightTransfer(t *testing.T) {
	if got := minutes("23:58", "00:08"); got != 10 {
		t.Fatalf("minutes() = %d, want 10", got)
	}
}

func TestMinutesHandlesSecondsFromKCI(t *testing.T) {
	if got := minutes("05:34:30", "06:02:15"); got != 27 {
		t.Fatalf("minutes() = %d, want 27", got)
	}
}

func TestSearchThresholdUsesJakartaTimeAndSelectedDay(t *testing.T) {
	now := time.Date(2026, 7, 30, 9, 15, 0, 0, time.FixedZone("WIB", 7*60*60))
	tests := []struct {
		name       string
		selected   string
		wantTime   string
		wantOffset int
		wantErr    bool
	}{
		{name: "now", wantTime: "09:15"},
		{name: "future today", selected: "10:30", wantTime: "10:30"},
		{name: "past means tomorrow", selected: "08:30", wantTime: "08:30", wantOffset: 1},
		{name: "same minute stays today", selected: "09:15", wantTime: "09:15"},
		{name: "invalid", selected: "9:15", wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			gotTime, gotOffset, err := searchThreshold(now, test.selected)
			if test.wantErr {
				if !errors.Is(err, ErrInvalidSearchTime) {
					t.Fatalf("searchThreshold() error = %v", err)
				}
				return
			}
			if err != nil || gotTime != test.wantTime || gotOffset != test.wantOffset {
				t.Fatalf("searchThreshold() = %q, %d, %v", gotTime, gotOffset, err)
			}
		})
	}
}

func TestSearchRollsToTheCorrectServiceDay(t *testing.T) {
	db := scheduleTestDB(t)
	svc := NewWithClock(repository.New(db), func() time.Time {
		return time.Date(2026, 7, 30, 10, 0, 0, 0, time.FixedZone("WIB", 7*60*60))
	})

	page, err := svc.Search(context.Background(), 1, 2, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Departures) != 1 || page.Departures[0].DayOffset != 1 {
		t.Fatalf("default rollover = %#v, want tomorrow", page.Departures)
	}

	page, err = svc.Search(context.Background(), 1, 2, "09:30")
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Departures) != 1 || page.Departures[0].DayOffset != 2 {
		t.Fatalf("past selected rollover = %#v, want day after tomorrow", page.Departures)
	}
}

func TestSavedSchedulesKeepsPartialResults(t *testing.T) {
	db := scheduleTestDB(t)
	svc := NewWithClock(repository.New(db), func() time.Time {
		return time.Date(2026, 7, 30, 7, 0, 0, 0, time.FixedZone("WIB", 7*60*60))
	})
	got := svc.SavedSchedules(context.Background(), []models.SavedRouteInput{
		{From: 1, To: 2},
		{From: 1, To: 3},
		{From: 1, To: 1},
		{From: 99, To: 2},
	})
	statuses := []string{"ok", "no_service", "invalid_route", "invalid_route"}
	for i, want := range statuses {
		if got[i].Status != want {
			t.Errorf("route %d status = %q, want %q", i, got[i].Status, want)
		}
	}
	if got[0].Next == nil || got[0].Next.Departure != "08:00" {
		t.Fatalf("next departure = %#v", got[0].Next)
	}
}

func TestTransfersCarryTheSecondLegIntoTheNextDay(t *testing.T) {
	db := scheduleTestDB(t)
	svc := NewWithClock(repository.New(db), time.Now)
	trips, err := svc.Transfers(
		context.Background(),
		models.Station{ID: 1, Name: "Alpha"},
		models.Station{ID: 3, Name: "Gamma"},
		"23:00",
		0,
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(trips) != 1 {
		t.Fatalf("transfers = %#v", trips)
	}
	if trips[0].First.DayOffset != 0 || trips[0].Second.DayOffset != 1 {
		t.Fatalf("leg offsets = %d, %d", trips[0].First.DayOffset, trips[0].Second.DayOffset)
	}
	if trips[0].WaitMinutes != 12 {
		t.Fatalf("wait = %d, want 12", trips[0].WaitMinutes)
	}
}

func scheduleTestDB(t *testing.T) *sql.DB {
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
			(3, 'CCC', 'Gamma', 'Blue'),
			(4, 'MRI', 'Manggarai', 'Blue');
		INSERT INTO trains VALUES
			(1, '1001', 'Alpha — Beta'),
			(2, '2001', 'Alpha — Manggarai'),
			(3, '3001', 'Manggarai — Gamma');
		INSERT INTO schedules VALUES
			(1, 1, '08:00', '08:00', 1),
			(1, 2, '08:30', '08:30', 2),
			(2, 1, '23:50', '23:50', 1),
			(2, 4, '23:58', '23:58', 2),
			(3, 4, '00:10', '00:10', 1),
			(3, 3, '00:30', '00:30', 2);
		INSERT INTO schedule_metadata VALUES
			('fetched_at', '2026-07-30T00:00:00Z'),
			('source_name', 'Test source');`)
	if err != nil {
		t.Fatal(err)
	}
	return db
}
