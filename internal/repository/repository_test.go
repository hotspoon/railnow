package repository

import (
	"context"
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

func TestDestinationsIncludesOneTransferRoute(t *testing.T) {
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
			(4, 'BOO', 'Bogor', 'Bogor');
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
	for _, want := range []string{"Tanah Abang", "Jurangmangu", "Bogor"} {
		if !got[want] {
			t.Errorf("destination %q was not selectable", want)
		}
	}
	if got["Manggarai"] {
		t.Error("origin station must not be selectable as its own destination")
	}
}

func TestClockTimeDropsKCISeconds(t *testing.T) {
	if got := clockTime("05:34:30"); got != "05:34" {
		t.Fatalf("clockTime() = %q, want 05:34", got)
	}
}
