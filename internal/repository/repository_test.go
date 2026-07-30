package repository

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/hotspoon/railnow/internal/models"
	_ "modernc.org/sqlite"
)

func TestDestinationsOnlyIncludesDirectRoute(t *testing.T) {
	db, err := sql.Open("sqlite", "file:destinations-test?mode=memory&cache=shared")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	_, err = db.Exec(`
		CREATE TABLE stations (id INTEGER PRIMARY KEY, code TEXT, name TEXT, line TEXT);
		CREATE TABLE schedules (train_id INTEGER, station_id INTEGER, sequence INTEGER);
		INSERT INTO stations VALUES
			(1, 'MRI', 'Manggarai', 'Bogor'),
			(2, 'THB', 'Tanah Abang', 'Rangkas Bitung'),
			(3, 'JMU', 'Jurangmangu', 'Rangkas Bitung'),
			(4, 'BOO', 'Bogor', 'Bogor'),
			(5, 'TNG', 'Tangerang', 'Tangerang');
		INSERT INTO schedules VALUES
			(10, 1, 1), (10, 2, 2),
			(20, 2, 1), (20, 3, 2),
			(30, 1, 1), (30, 4, 2);`)
	if err != nil {
		t.Fatal(err)
	}

	destinations, err := New(db).Destinations(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, station := range destinations {
		got[station.Name] = true
	}
	for _, want := range []string{"Tanah Abang", "Bogor"} {
		if !got[want] {
			t.Errorf("destination %q was not selectable", want)
		}
	}
	if got["Manggarai"] {
		t.Error("origin station must not be selectable as its own destination")
	}
	if got["Jurangmangu"] {
		t.Error("one-transfer station must not be shown in the direct destination picker")
	}
	if got["Tangerang"] {
		t.Error("unreachable station must not be shown in the direct destination picker")
	}
}

func TestClockTimeDropsKCISeconds(t *testing.T) {
	if got := clockTime("05:34:30"); got != "05:34" {
		t.Fatalf("clockTime() = %q, want 05:34", got)
	}
}

func TestApplyScheduleFreshness(t *testing.T) {
	now := time.Date(2026, 7, 30, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name       string
		fetchedAt  string
		wantStatus string
		wantStale  bool
	}{
		{name: "available", fetchedAt: "2026-07-29T12:00:00Z", wantStatus: "available"},
		{name: "stale", fetchedAt: "2026-06-01T12:00:00Z", wantStatus: "stale", wantStale: true},
		{name: "invalid", fetchedAt: "not-a-date", wantStatus: "unknown"},
		{name: "missing", wantStatus: "unknown"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			info := models.ScheduleInfo{FetchedAt: test.fetchedAt}
			applyScheduleFreshness(&info, now)
			if info.Status != test.wantStatus || info.Stale != test.wantStale {
				t.Fatalf("freshness = %q, stale=%t", info.Status, info.Stale)
			}
			if test.wantStatus == "available" && info.UpdatedLabel != "29 Jul 2026, 19:00 WIB" {
				t.Fatalf("UpdatedLabel = %q", info.UpdatedLabel)
			}
		})
	}
}
