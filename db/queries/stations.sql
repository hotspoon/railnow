-- name: ListStations :many
SELECT id, code, name, line FROM stations ORDER BY name;

-- name: GetStationByID :one
SELECT id, code, name, line FROM stations WHERE id = ?;
