// Command refresh-kci replaces the Turso timetable with a validated KCI API snapshot.
package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/hotspoon/railnow/internal/database"
	"github.com/hotspoon/railnow/internal/kci"
	"github.com/pressly/goose/v3"
)

const insertBatchSize = 100

func main() {
	endpoint := flag.String("endpoint", kci.DefaultEndpoint, "KCI public schedule API endpoint")
	stationsEndpoint := flag.String("stations-endpoint", kci.DefaultStationsEndpoint, "KCI public station catalog endpoint")
	out := flag.String("out", ".cache/kci", "directory for the raw API snapshot")
	delay := flag.Duration("delay", 500*time.Millisecond, "delay between station requests")
	flag.Parse()

	remoteURL := os.Getenv("TURSO_DATABASE_URL")
	if remoteURL == "" {
		log.Fatal("TURSO_DATABASE_URL is required; refresh-kci never falls back to local SQLite")
	}
	db, err := database.OpenTurso(remoteURL, os.Getenv("TURSO_AUTH_TOKEN"))
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	if err := goose.SetDialect("sqlite3"); err != nil {
		log.Fatal(err)
	}
	if err := goose.Up(db, "db/migrations"); err != nil {
		log.Fatalf("apply Turso migrations: %v", err)
	}
	jakarta, _ := time.LoadLocation("Asia/Jakarta")
	progressPath := filepath.Join(*out, "kci-progress-"+time.Now().In(jakarta).Format("20060102")+".json")
	progress, err := loadProgress(progressPath)
	if err != nil {
		log.Fatalf("read saved refresh progress: %v", err)
	}
	client := kci.Client{
		Endpoint:         *endpoint,
		StationsEndpoint: *stationsEndpoint,
		Delay:            *delay,
		Retries:          3,
		Progress: func(current, total int, station kci.Station) {
			if current == 1 || current == total || current%10 == 0 {
				log.Printf("Fetching station %d/%d: %s", current, total, station.Code)
			}
		},
		Initial: progress,
		SaveProgress: func(snapshot kci.Snapshot) error {
			return writeJSONAtomic(progressPath, snapshot)
		},
	}
	sourceStations, err := client.FetchStations(context.Background())
	if err != nil {
		log.Fatalf("fetch KCI station catalog: %v", err)
	}
	log.Printf("Fetched %d enabled stations from KCI", len(sourceStations))
	if completed := len(progress.Stations) + len(progress.Unsupported); completed > 0 {
		log.Printf("Resuming saved snapshot: %d/%d stations already fetched", completed, len(sourceStations))
	}
	snapshot, err := client.Fetch(context.Background(), sourceStations)
	if err != nil {
		log.Fatal(err)
	}
	stations, destinationOnly := kci.AddDestinationStations(snapshot, sourceStations)
	if len(destinationOnly) > 0 {
		log.Printf("Added %d destination-only stations from the schedule API", len(destinationOnly))
	}
	trains, err := kci.Normalize(snapshot, stations)
	if err != nil {
		log.Fatal(err)
	}
	if err := validate(sourceStations, snapshot, trains); err != nil {
		log.Fatal(err)
	}
	raw, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		log.Fatal(err)
	}
	if err := writeSnapshot(*out, snapshot.FetchedAt, raw); err != nil {
		log.Fatal(err)
	}
	if err := replaceTimetable(context.Background(), db, stations, trains, snapshot.FetchedAt, raw, *endpoint, *stationsEndpoint, snapshot.Unsupported); err != nil {
		log.Fatal(err)
	}
	if err := os.Remove(progressPath); err != nil && !os.IsNotExist(err) {
		log.Printf("warning: remove completed progress snapshot: %v", err)
	}
	log.Printf("Refreshed Turso with %d supported stations, %d trains, and %d scheduled stops", len(snapshot.Stations), len(trains), stopCount(trains))
}

func validate(stations []kci.Station, snapshot kci.Snapshot, trains []kci.Train) error {
	if len(snapshot.Stations)+len(snapshot.Unsupported) != len(stations) {
		return fmt.Errorf("incomplete snapshot: got %d supported and %d unsupported of %d stations", len(snapshot.Stations), len(snapshot.Unsupported), len(stations))
	}
	if len(trains) == 0 || stopCount(trains) < len(snapshot.Stations)*2 {
		return fmt.Errorf("incomplete timetable: got %d trains and %d stops", len(trains), stopCount(trains))
	}
	return nil
}

func writeSnapshot(directory string, fetchedAt time.Time, raw []byte) error {
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return err
	}
	path := filepath.Join(directory, "kci-snapshot-"+fetchedAt.UTC().Format("20060102T150405Z")+".json")
	return writeFileAtomic(path, raw)
}

func loadProgress(path string) (kci.Snapshot, error) {
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return kci.Snapshot{}, nil
	}
	if err != nil {
		return kci.Snapshot{}, err
	}
	var snapshot kci.Snapshot
	if err := json.Unmarshal(raw, &snapshot); err != nil {
		return kci.Snapshot{}, fmt.Errorf("decode %s: %w", path, err)
	}
	return snapshot, nil
}

func writeJSONAtomic(path string, value any) error {
	raw, err := json.Marshal(value)
	if err != nil {
		return err
	}
	return writeFileAtomic(path, raw)
}

func writeFileAtomic(path string, raw []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	temp := path + ".tmp"
	if err := os.WriteFile(temp, raw, 0o600); err != nil {
		return err
	}
	return os.Rename(temp, path)
}

func replaceTimetable(ctx context.Context, db *sql.DB, stations []kci.Station, trains []kci.Train, fetchedAt time.Time, raw []byte, sourceURL, stationsURL string, unsupported []string) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, station := range stations {
		if _, err := tx.ExecContext(ctx, `INSERT INTO stations(code,name,line) VALUES(?,?,?) ON CONFLICT(code) DO UPDATE SET name=excluded.name,line=excluded.line`, station.Code, station.Name, station.Line); err != nil {
			return fmt.Errorf("upsert station %s: %w", station.Code, err)
		}
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM schedules"); err != nil {
		return fmt.Errorf("clear schedules: %w", err)
	}
	if _, err := tx.ExecContext(ctx, "DELETE FROM trains"); err != nil {
		return fmt.Errorf("clear trains: %w", err)
	}
	stationIDs, err := stationIDs(ctx, tx)
	if err != nil {
		return err
	}
	if err := insertTrains(ctx, tx, trains); err != nil {
		return err
	}
	trainIDs, err := trainIDs(ctx, tx)
	if err != nil {
		return err
	}
	if err := insertStops(ctx, tx, trains, trainIDs, stationIDs); err != nil {
		return err
	}
	hash := sha256.Sum256(raw)
	jakarta, _ := time.LoadLocation("Asia/Jakarta")
	metadata := map[string]string{
		"source":               "KAI Commuter API schedule snapshot",
		"source_name":          "KAI Commuter public schedule API",
		"source_url":           sourceURL,
		"station_source_url":   stationsURL,
		"snapshot_date":        fetchedAt.In(jakarta).Format("02 Jan 2006"),
		"effective_date":       fetchedAt.In(jakarta).Format("02 Jan 2006"),
		"fetched_at":           fetchedAt.UTC().Format(time.RFC3339),
		"day_type":             "Schedule snapshot for the refresh date",
		"limitations":          "Scheduled timetable snapshot only; not real-time train position or delay data.",
		"station_count":        fmt.Sprint(len(stations)),
		"train_count":          fmt.Sprint(len(trains)),
		"stop_count":           fmt.Sprint(stopCount(trains)),
		"snapshot_sha256":      hex.EncodeToString(hash[:]),
		"unsupported_stations": strings.Join(unsupported, ","),
	}
	keys := make([]string, 0, len(metadata))
	for key := range metadata {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if _, err := tx.ExecContext(ctx, `INSERT INTO schedule_metadata(key,value) VALUES(?,?) ON CONFLICT(key) DO UPDATE SET value=excluded.value`, key, metadata[key]); err != nil {
			return fmt.Errorf("update metadata %s: %w", key, err)
		}
	}
	return tx.Commit()
}

func stationIDs(ctx context.Context, tx *sql.Tx) (map[string]int64, error) {
	rows, err := tx.QueryContext(ctx, "SELECT id, code FROM stations")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := map[string]int64{}
	for rows.Next() {
		var id int64
		var code string
		if err := rows.Scan(&id, &code); err != nil {
			return nil, err
		}
		ids[code] = id
	}
	return ids, rows.Err()
}

func insertTrains(ctx context.Context, tx *sql.Tx, trains []kci.Train) error {
	for start := 0; start < len(trains); start += insertBatchSize {
		end := min(start+insertBatchSize, len(trains))
		args := make([]any, 0, (end-start)*2)
		values := make([]string, 0, end-start)
		for _, train := range trains[start:end] {
			values = append(values, "(?,?)")
			args = append(args, train.Number, train.Route)
		}
		if _, err := tx.ExecContext(ctx, "INSERT INTO trains(train_number,route_name) VALUES "+strings.Join(values, ","), args...); err != nil {
			return fmt.Errorf("insert train batch: %w", err)
		}
	}
	return nil
}

func trainIDs(ctx context.Context, tx *sql.Tx) (map[string]int64, error) {
	rows, err := tx.QueryContext(ctx, "SELECT id, train_number FROM trains")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := map[string]int64{}
	for rows.Next() {
		var id int64
		var number string
		if err := rows.Scan(&id, &number); err != nil {
			return nil, err
		}
		ids[number] = id
	}
	return ids, rows.Err()
}

func insertStops(ctx context.Context, tx *sql.Tx, trains []kci.Train, trainIDs, stationIDs map[string]int64) error {
	args := make([]any, 0, insertBatchSize*6)
	values := make([]string, 0, insertBatchSize)
	flush := func() error {
		if len(values) == 0 {
			return nil
		}
		_, err := tx.ExecContext(ctx, "INSERT INTO schedules(train_id,station_id,arrival,departure,sequence,arrival_estimated) VALUES "+strings.Join(values, ","), args...)
		args = args[:0]
		values = values[:0]
		return err
	}
	for _, train := range trains {
		trainID, ok := trainIDs[train.Number]
		if !ok {
			return fmt.Errorf("missing inserted train %s", train.Number)
		}
		for sequence, stop := range train.Stops {
			stationID, ok := stationIDs[stop.StationCode]
			if !ok {
				return fmt.Errorf("unknown station %s for train %s", stop.StationCode, train.Number)
			}
			values = append(values, "(?,?,?,?,?,?)")
			args = append(args, trainID, stationID, stop.Time, stop.Time, sequence+1, true)
			if len(values) == insertBatchSize {
				if err := flush(); err != nil {
					return fmt.Errorf("insert schedule batch: %w", err)
				}
			}
		}
	}
	if err := flush(); err != nil {
		return fmt.Errorf("insert final schedule batch: %w", err)
	}
	return nil
}

func stopCount(trains []kci.Train) int {
	count := 0
	for _, train := range trains {
		count += len(train.Stops)
	}
	return count
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
