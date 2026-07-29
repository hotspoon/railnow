-- +goose Up
CREATE TABLE stations (
  id INTEGER PRIMARY KEY,
  code TEXT NOT NULL UNIQUE,
  name TEXT NOT NULL,
  line TEXT NOT NULL
);

CREATE TABLE trains (
  id INTEGER PRIMARY KEY,
  train_number TEXT NOT NULL UNIQUE,
  route_name TEXT NOT NULL
);

CREATE TABLE schedules (
  id INTEGER PRIMARY KEY,
  train_id INTEGER NOT NULL REFERENCES trains(id),
  station_id INTEGER NOT NULL REFERENCES stations(id),
  arrival TEXT NOT NULL,
  departure TEXT NOT NULL,
  sequence INTEGER NOT NULL,
  UNIQUE(train_id, station_id)
);

CREATE TABLE favorites (
  id INTEGER PRIMARY KEY,
  from_station INTEGER NOT NULL REFERENCES stations(id),
  to_station INTEGER NOT NULL REFERENCES stations(id),
  label TEXT NOT NULL DEFAULT 'Favorite',
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  UNIQUE(from_station, to_station)
);

CREATE TABLE searches (
  id INTEGER PRIMARY KEY,
  from_station INTEGER NOT NULL REFERENCES stations(id),
  to_station INTEGER NOT NULL REFERENCES stations(id),
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX schedules_station_sequence_idx ON schedules(station_id, sequence);
CREATE INDEX searches_created_at_idx ON searches(created_at DESC);

-- +goose Down
DROP TABLE searches;
DROP TABLE favorites;
DROP TABLE schedules;
DROP TABLE trains;
DROP TABLE stations;
