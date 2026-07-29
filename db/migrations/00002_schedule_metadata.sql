-- +goose Up
ALTER TABLE schedules ADD COLUMN arrival_estimated INTEGER NOT NULL DEFAULT 1;
CREATE TABLE schedule_metadata (key TEXT PRIMARY KEY, value TEXT NOT NULL);

-- +goose Down
DROP TABLE schedule_metadata;
