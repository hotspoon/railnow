// Command importer loads KAI Commuter schedule CSV data into RailNow's SQLite database.
package main

import (
	"database/sql"
	"encoding/csv"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite"
)

type station struct{ Code, Name string }
type departure struct {
	StationCode, Time string
	ArrivalEstimated  bool
}
type train struct {
	Number, Route, DestinationCode, DestinationTime string
	Stops                                           []departure
}

func main() {
	input := flag.String("input", "", "Path to commuter_line_schedule.csv")
	stationsPath := flag.String("stations", "", "Path to dim_station.csv (defaults beside --input)")
	database := flag.String("database", "data/railnow.db", "SQLite database path")
	replace := flag.Bool("replace", false, "Replace all existing stations, schedules, searches, and favorites")
	flag.Parse()
	if *input == "" {
		log.Fatal("--input is required")
	}
	if *stationsPath == "" {
		*stationsPath = filepath.Join(filepath.Dir(*input), "dim_station.csv")
	}

	stations, err := readStations(*stationsPath)
	if err != nil {
		log.Fatal(err)
	}
	trains, err := readSchedules(*input)
	if err != nil {
		log.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(*database), 0o755); err != nil {
		log.Fatal(err)
	}
	db, err := sql.Open("sqlite", *database)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	if err := goose.SetDialect("sqlite3"); err != nil {
		log.Fatal(err)
	}
	if err := goose.Up(db, "db/migrations"); err != nil {
		log.Fatal(err)
	}
	if err := importData(db, stations, trains, *replace); err != nil {
		log.Fatal(err)
	}
	log.Printf("Imported %d stations and %d train journeys from %s", len(stations), len(trains), *input)
}

func readStations(path string) (map[string]station, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	r := csv.NewReader(f)
	if _, err := r.Read(); err != nil {
		return nil, err
	}
	result := map[string]station{}
	for {
		row, err := r.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		if len(row) < 2 {
			continue
		}
		result[row[0]] = station{Code: row[0], Name: displayName(row[1])}
	}
	return result, nil
}

func readSchedules(path string) ([]train, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	r := csv.NewReader(f)
	header, err := r.Read()
	if err != nil {
		return nil, err
	}
	index := map[string]int{}
	for i, name := range header {
		index[name] = i
	}
	for _, field := range []string{"train_id", "route_name", "departure_time_utc7", "station_departure_id", "station_destination_id", "destination_time_utc7"} {
		if _, ok := index[field]; !ok {
			return nil, fmt.Errorf("CSV is missing %q", field)
		}
	}
	grouped := map[string]*train{}
	for {
		row, err := r.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		if len(row) != len(header) {
			continue
		}
		number, route := row[index["train_id"]], row[index["route_name"]]
		key := number + "|" + route
		if grouped[key] == nil {
			grouped[key] = &train{Number: number, Route: displayRoute(route), DestinationCode: row[index["station_destination_id"]], DestinationTime: strings.TrimSuffix(row[index["destination_time_utc7"]], ":00")}
		}
		grouped[key].Stops = append(grouped[key].Stops, departure{StationCode: row[index["station_departure_id"]], Time: strings.TrimSuffix(row[index["departure_time_utc7"]], ":00"), ArrivalEstimated: true})
	}
	result := make([]train, 0, len(grouped))
	for _, item := range grouped {
		terminalExists := false
		for _, stop := range item.Stops {
			if stop.StationCode == item.DestinationCode {
				terminalExists = true
				break
			}
		}
		if !terminalExists && item.DestinationCode != "" && item.DestinationTime != "" {
			item.Stops = append(item.Stops, departure{StationCode: item.DestinationCode, Time: item.DestinationTime, ArrivalEstimated: false})
		}
		item.Stops = chronologicalStops(item.Stops)
		result = append(result, *item)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Number < result[j].Number })
	return result, nil
}

func chronologicalStops(stops []departure) []departure {
	if len(stops) < 2 {
		return stops
	}
	sort.Slice(stops, func(i, j int) bool { return stops[i].Time < stops[j].Time })
	start, largestGap := 0, -1
	for i := range stops {
		current := clockMinutes(stops[i].Time)
		next := clockMinutes(stops[(i+1)%len(stops)].Time)
		if i == len(stops)-1 {
			next += 24 * 60
		}
		if gap := next - current; gap > largestGap {
			largestGap, start = gap, (i+1)%len(stops)
		}
	}
	return append(append([]departure{}, stops[start:]...), stops[:start]...)
}

func clockMinutes(value string) int {
	var hours, minutes int
	fmt.Sscanf(value, "%d:%d", &hours, &minutes)
	return hours*60 + minutes
}

func displayRoute(route string) string {
	parts := strings.Split(route, "-")
	for i := range parts {
		parts[i] = displayName(parts[i])
	}
	return strings.Join(parts, " → ")
}

func importData(db *sql.DB, stations map[string]station, trains []train, replace bool) error {
	var count int
	if err := db.QueryRow("SELECT COUNT(*) FROM stations").Scan(&count); err != nil {
		return err
	}
	if count > 0 && !replace {
		return errors.New("database already has data; rerun with --replace to overwrite it")
	}
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.Exec("PRAGMA foreign_keys = ON"); err != nil {
		return err
	}
	if replace {
		for _, table := range []string{"searches", "favorites", "schedules", "trains", "stations"} {
			if _, err = tx.Exec("DELETE FROM " + table); err != nil {
				return err
			}
		}
	}
	for key, value := range map[string]string{"source": "dedewanta/scraping-krl-jabodetabek (KAI Commuter API snapshot)", "snapshot_date": "24 Mar 2024", "day_type": "Jenis hari tidak diketahui", "limitations": "Bukan data real-time; waktu tiba perantara diestimasi dari waktu berangkat."} {
		if _, err = tx.Exec(`INSERT INTO schedule_metadata(key,value) VALUES(?,?) ON CONFLICT(key) DO UPDATE SET value=excluded.value`, key, value); err != nil {
			return err
		}
	}
	stationIDs := map[string]int64{}
	for _, item := range stations {
		result, err := tx.Exec("INSERT INTO stations(code,name,line) VALUES(?,?,?)", item.Code, item.Name, "KAI Commuter")
		if err != nil {
			return err
		}
		id, _ := result.LastInsertId()
		stationIDs[item.Code] = id
	}
	for _, item := range trains {
		if len(item.Stops) < 2 {
			continue
		}
		result, err := tx.Exec("INSERT INTO trains(train_number,route_name) VALUES(?,?)", item.Number, item.Route)
		if err != nil {
			return err
		}
		trainID, _ := result.LastInsertId()
		for sequence, stop := range item.Stops {
			stationID, ok := stationIDs[stop.StationCode]
			if !ok {
				return fmt.Errorf("unknown station code %q in train %s", stop.StationCode, item.Number)
			}
			if _, err = tx.Exec("INSERT INTO schedules(train_id,station_id,arrival,departure,sequence,arrival_estimated) VALUES(?,?,?,?,?,?)", trainID, stationID, stop.Time, stop.Time, sequence+1, stop.ArrivalEstimated); err != nil {
				return err
			}
		}
	}
	return tx.Commit()
}

func displayName(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	words := strings.Fields(value)
	for i, word := range words {
		if word == "ui" {
			words[i] = "UI"
		} else {
			words[i] = strings.ToUpper(word[:1]) + word[1:]
		}
	}
	name := strings.Join(words, " ")
	replacements := map[string]string{
		"Bandarasoekarnohatta": "Bandara Soekarno-Hatta",
		"Bni City":             "BNI City",
		"Bojonggede":           "Bojong Gede",
		"Gondang Dia":          "Gondangdia",
		"Jakartakota":          "Jakarta Kota",
		"Kampungbandan":        "Kampung Bandan",
		"Parungpanjang":        "Parung Panjang",
		"Rangkasbitung":        "Rangkas Bitung",
		"Tanahabang":           "Tanah Abang",
		"Tanahtinggi":          "Tanah Tinggi",
		"Tanjungbarat":         "Tanjung Barat",
		"Telagamurni":          "Telaga Murni",
		"Tanjungpriuk":         "Tanjung Priok",
		"Universits Pancasila": "Universitas Pancasila",
	}
	if replacement, ok := replacements[name]; ok {
		return replacement
	}
	return name
}
