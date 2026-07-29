package repository

import (
	"context"
	"database/sql"
	"github.com/hotspoon/railnow/internal/models"
	"strings"
	"time"
)

type Repository struct{ db *sql.DB }

func New(db *sql.DB) *Repository { return &Repository{db: db} }

func (r *Repository) Stations(ctx context.Context) ([]models.Station, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id, code, name, line FROM stations WHERE EXISTS (SELECT 1 FROM schedules WHERE schedules.station_id = stations.id) ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.Station
	for rows.Next() {
		var s models.Station
		if err := rows.Scan(&s.ID, &s.Code, &s.Name, &s.Line); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}
func (r *Repository) Station(ctx context.Context, id int64) (models.Station, error) {
	var s models.Station
	err := r.db.QueryRowContext(ctx, `SELECT id,code,name,line FROM stations WHERE id=?`, id).Scan(&s.ID, &s.Code, &s.Name, &s.Line)
	return s, err
}
func (r *Repository) Destinations(ctx context.Context, from int64) ([]models.Station, error) {
	// A destination is selectable when it is reachable directly or with one
	// interchange at one of RailNow's supported hubs. This keeps the picker
	// useful for routes such as Manggarai → Jurangmangu without exposing every
	// station as a dead-end option.
	rows, err := r.db.QueryContext(ctx, `WITH hubs AS (
			SELECT id FROM stations WHERE LOWER(name) IN ('manggarai', 'tanah abang', 'duri', 'jatinegara')
		), direct AS (
			SELECT DISTINCT destination.station_id
			FROM schedules origin
			JOIN schedules destination ON destination.train_id = origin.train_id AND destination.sequence > origin.sequence
			WHERE origin.station_id = ?
		), one_transfer AS (
			SELECT DISTINCT second_destination.station_id
			FROM schedules first_origin
			JOIN schedules first_destination ON first_destination.train_id = first_origin.train_id AND first_destination.sequence > first_origin.sequence
			JOIN hubs ON hubs.id = first_destination.station_id
			JOIN schedules second_origin ON second_origin.station_id = hubs.id
			JOIN schedules second_destination ON second_destination.train_id = second_origin.train_id AND second_destination.sequence > second_origin.sequence
			WHERE first_origin.station_id = ?
		)
		SELECT s.id, s.code, s.name, s.line
		FROM stations s
		JOIN (SELECT station_id FROM direct UNION SELECT station_id FROM one_transfer) selectable ON selectable.station_id = s.id
		WHERE s.id <> ?
		ORDER BY s.name`, from, from, from)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.Station
	for rows.Next() {
		var s models.Station
		if err := rows.Scan(&s.ID, &s.Code, &s.Name, &s.Line); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}
func (r *Repository) Departures(ctx context.Context, from, to int64, fromTime string) ([]models.Departure, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT t.id,t.train_number,t.route_name,o.departure,d.arrival FROM schedules o JOIN schedules d ON d.train_id=o.train_id JOIN trains t ON t.id=o.train_id WHERE o.station_id=? AND d.station_id=? AND o.sequence<d.sequence AND (? = '' OR o.departure >= ?) ORDER BY o.departure`, from, to, fromTime, fromTime)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.Departure
	for rows.Next() {
		var d models.Departure
		if err := rows.Scan(&d.TrainID, &d.Number, &d.Route, &d.Departure, &d.Arrival); err != nil {
			return nil, err
		}
		d.Departure = clockTime(d.Departure)
		d.Arrival = clockTime(d.Arrival)
		d.Duration = minutes(d.Departure, d.Arrival)
		out = append(out, d)
	}
	return out, rows.Err()
}
func (r *Repository) Stops(ctx context.Context, trainID int64) ([]models.Stop, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT s.name,sc.arrival,sc.departure,sc.sequence FROM schedules sc JOIN stations s ON s.id=sc.station_id WHERE sc.train_id=? ORDER BY sc.sequence`, trainID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.Stop
	for rows.Next() {
		var s models.Stop
		if err := rows.Scan(&s.Name, &s.Arrival, &s.Departure, &s.Sequence); err != nil {
			return nil, err
		}
		s.Arrival = clockTime(s.Arrival)
		s.Departure = clockTime(s.Departure)
		out = append(out, s)
	}
	return out, rows.Err()
}

// clockTime accepts the KCI API's HH:MM:SS values and returns the HH:MM
// representation used everywhere in the interface and timetable calculations.
func clockTime(value string) string {
	value = strings.TrimSpace(value)
	for _, format := range []string{"15:04:05", "15:04"} {
		parsed, err := time.Parse(format, value)
		if err == nil {
			return parsed.Format("15:04")
		}
	}
	return value
}
func (r *Repository) RecordSearch(ctx context.Context, from, to int64) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO searches(from_station,to_station) VALUES(?,?)`, from, to)
	return err
}
func (r *Repository) Recents(ctx context.Context) ([]models.Favorite, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT 0,'',a.name,b.name,a.id,b.id FROM searches x JOIN stations a ON a.id=x.from_station JOIN stations b ON b.id=x.to_station GROUP BY a.id,b.id ORDER BY MAX(x.created_at) DESC LIMIT 3`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.Favorite
	for rows.Next() {
		var f models.Favorite
		if err := rows.Scan(&f.ID, &f.Label, &f.From, &f.To, &f.FromID, &f.ToID); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}
func (r *Repository) Favorites(ctx context.Context) ([]models.Favorite, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT f.id,f.label,a.name,b.name,a.id,b.id FROM favorites f JOIN stations a ON a.id=f.from_station JOIN stations b ON b.id=f.to_station ORDER BY f.created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.Favorite
	for rows.Next() {
		var f models.Favorite
		if err := rows.Scan(&f.ID, &f.Label, &f.From, &f.To, &f.FromID, &f.ToID); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}
func (r *Repository) ToggleFavorite(ctx context.Context, from, to int64) (bool, error) {
	var id int64
	err := r.db.QueryRowContext(ctx, `SELECT id FROM favorites WHERE from_station=? AND to_station=?`, from, to).Scan(&id)
	if err == nil {
		_, err = r.db.ExecContext(ctx, `DELETE FROM favorites WHERE id=?`, id)
		return false, err
	}
	if err != sql.ErrNoRows {
		return false, err
	}
	_, err = r.db.ExecContext(ctx, `INSERT INTO favorites(from_station,to_station,label) VALUES(?,?,'My route')`, from, to)
	return err == nil, err
}
func (r *Repository) IsFavorite(ctx context.Context, from, to int64) (bool, error) {
	var x int
	err := r.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM favorites WHERE from_station=? AND to_station=?)`, from, to).Scan(&x)
	return x == 1, err
}

func (r *Repository) ScheduleInfo(ctx context.Context) (models.ScheduleInfo, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT key, value FROM schedule_metadata WHERE key IN ('snapshot_date', 'day_type', 'effective_date', 'source_name', 'source_url', 'fetched_at')`)
	if err != nil {
		return models.ScheduleInfo{}, err
	}
	defer rows.Close()
	info := models.ScheduleInfo{DayType: "Jenis hari tidak diketahui"}
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			return info, err
		}
		if key == "snapshot_date" {
			info.SnapshotDate = value
		}
		if key == "day_type" {
			info.DayType = value
		}
		if key == "effective_date" {
			info.EffectiveDate = value
		}
		if key == "source_name" {
			info.SourceName = value
		}
		if key == "source_url" {
			info.SourceURL = value
		}
		if key == "fetched_at" {
			info.FetchedAt = value
		}
	}
	if err := rows.Err(); err != nil {
		return info, err
	}
	if info.FetchedAt != "" {
		if fetched, err := time.Parse(time.RFC3339, info.FetchedAt); err == nil {
			info.Stale = time.Since(fetched) > 30*24*time.Hour
		}
	} else if snapshot, err := time.Parse("02 Jan 2006", info.SnapshotDate); err == nil {
		info.Stale = time.Since(snapshot) > 30*24*time.Hour
	}
	return info, nil
}
func minutes(a, b string) int {
	base := "2006-01-02 "
	ta, _ := time.Parse("2006-01-02 15:04", base+a)
	tb, _ := time.Parse("2006-01-02 15:04", base+b)
	if tb.Before(ta) {
		tb = tb.Add(24 * time.Hour)
	}
	return int(tb.Sub(ta).Minutes())
}
