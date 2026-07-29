-- name: SearchDepartures :many
SELECT t.id, t.train_number, t.route_name, origin.departure AS departure,
       destination.arrival AS arrival
FROM schedules origin
JOIN schedules destination ON destination.train_id = origin.train_id
JOIN trains t ON t.id = origin.train_id
WHERE origin.station_id = ? AND destination.station_id = ?
  AND origin.sequence < destination.sequence
ORDER BY origin.departure;
