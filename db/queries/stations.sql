-- name: ListStations :many
SELECT id, code, name, line FROM stations WHERE EXISTS (SELECT 1 FROM schedules WHERE schedules.station_id = stations.id) ORDER BY name;

-- name: GetStationByID :one
SELECT id, code, name, line FROM stations WHERE id = ?;
