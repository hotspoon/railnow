-- +goose Up
-- Supports origin/destination searches and ordering upcoming departures without
-- scanning the whole timetable for each request.
CREATE INDEX schedules_station_departure_train_idx ON schedules(station_id, departure, train_id, sequence);
CREATE INDEX schedules_train_sequence_idx ON schedules(train_id, sequence);

-- +goose Down
DROP INDEX schedules_train_sequence_idx;
DROP INDEX schedules_station_departure_train_idx;
