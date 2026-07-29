package main

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"github.com/hotspoon/railnow/internal/database"
	"github.com/hotspoon/railnow/internal/kci"
)

func TestReplaceTimetableIsAtomicAndPreservesLegacyData(t *testing.T) {
	db, err := database.OpenSQLite(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	createRefreshSchema(t, db)
	if _, err := db.Exec(`INSERT INTO stations(id,code,name,line) VALUES(1,'A','Alpha','Line'),(2,'B','Beta','Line'); INSERT INTO trains(id,train_number,route_name) VALUES(1,'old','Old'); INSERT INTO schedules(train_id,station_id,arrival,departure,sequence,arrival_estimated) VALUES(1,1,'01:00','01:00',1,1); INSERT INTO favorites(from_station,to_station,label) VALUES(1,2,'Saved'); INSERT INTO searches(from_station,to_station) VALUES(1,2)`); err != nil {
		t.Fatal(err)
	}
	stations := []kci.Station{{ID: 1, Code: "A", Name: "Alpha", Line: "Line"}, {ID: 2, Code: "B", Name: "Beta", Line: "Line"}}
	trains := []kci.Train{{Number: "new", Route: "Alpha → Beta", Stops: []kci.Stop{{StationCode: "A", Time: "08:00"}, {StationCode: "B", Time: "08:10"}}}}
	if err := replaceTimetable(context.Background(), db, stations, trains, time.Date(2026, 7, 29, 0, 0, 0, 0, time.UTC), []byte(`{"source":"test"}`), "https://example.test/schedules", "https://example.test/stations", nil); err != nil {
		t.Fatal(err)
	}
	assertCount(t, db, "trains", 1)
	assertCount(t, db, "schedules", 2)
	assertCount(t, db, "favorites", 1)
	assertCount(t, db, "searches", 1)
	assertCount(t, db, "schedule_metadata", 14)
}

func TestValidateRejectsIncompleteSnapshot(t *testing.T) {
	err := validate([]kci.Station{{Code: "A"}, {Code: "B"}}, kci.Snapshot{Stations: map[string][]kci.APIRecord{"A": {}}}, nil)
	if err == nil {
		t.Fatal("validate accepted incomplete snapshot")
	}
}

func createRefreshSchema(t *testing.T, db *sql.DB) {
	t.Helper()
	_, err := db.Exec(`
CREATE TABLE stations (id INTEGER PRIMARY KEY, code TEXT NOT NULL UNIQUE, name TEXT NOT NULL, line TEXT NOT NULL);
CREATE TABLE trains (id INTEGER PRIMARY KEY, train_number TEXT NOT NULL UNIQUE, route_name TEXT NOT NULL);
CREATE TABLE schedules (id INTEGER PRIMARY KEY, train_id INTEGER NOT NULL, station_id INTEGER NOT NULL, arrival TEXT NOT NULL, departure TEXT NOT NULL, sequence INTEGER NOT NULL, arrival_estimated INTEGER NOT NULL, UNIQUE(train_id, station_id));
CREATE TABLE favorites (id INTEGER PRIMARY KEY, from_station INTEGER NOT NULL, to_station INTEGER NOT NULL, label TEXT NOT NULL);
CREATE TABLE searches (id INTEGER PRIMARY KEY, from_station INTEGER NOT NULL, to_station INTEGER NOT NULL);
CREATE TABLE schedule_metadata (key TEXT PRIMARY KEY, value TEXT NOT NULL);`)
	if err != nil {
		t.Fatal(err)
	}
}

func assertCount(t *testing.T, db *sql.DB, table string, want int) {
	t.Helper()
	var got int
	if err := db.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&got); err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("%s = %d, want %d", table, got, want)
	}
}
