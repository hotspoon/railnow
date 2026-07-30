// Package kci fetches and normalizes published KAI Commuter timetable data.
package kci

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"
	"unicode"
)

const (
	DefaultEndpoint         = "https://kci.id/api/krl/schedules"
	DefaultStationsEndpoint = "https://kci.id/api/krl/stations"
)

// ErrUnsupportedStation is returned when a catalog code is not served by KCI.
var ErrUnsupportedStation = errors.New("station is not supported by the KCI schedule API")

// Station is the stable station catalog stored by JadwalKRL.
type Station struct {
	ID   int64
	Code string
	Name string
	Line string
}

// APIRecord is one published departure from the KCI schedule API.
type APIRecord struct {
	TrainID   string `json:"train_id"`
	Name      string `json:"ka_name"`
	RouteName string `json:"route_name"`
	Dest      string `json:"dest"`
	Time      string `json:"time_est"`
	DestTime  string `json:"dest_time"`
}

type apiResponse struct {
	Status int         `json:"status"`
	Data   []APIRecord `json:"data"`
}

type stationResponse struct {
	Status int `json:"status"`
	Data   []struct {
		ID      string `json:"sta_id"`
		Name    string `json:"sta_name"`
		Enabled int    `json:"fg_enable"`
	} `json:"data"`
}

// Snapshot is retained locally for diagnostics only. It contains no credentials.
type Snapshot struct {
	FetchedAt   time.Time              `json:"fetched_at"`
	Stations    map[string][]APIRecord `json:"stations"`
	Unsupported []string               `json:"unsupported_stations,omitempty"`
}

// Client fetches the public KCI endpoint sequentially to avoid burdening it.
type Client struct {
	Endpoint         string
	StationsEndpoint string
	HTTP             *http.Client
	Delay            time.Duration
	Retries          int
	Sleep            func(time.Duration)
	Progress         func(current, total int, station Station)
	Initial          Snapshot
	SaveProgress     func(Snapshot) error
}

// FetchStations reads KCI's own station catalog and excludes group headers.
func (c Client) FetchStations(ctx context.Context) ([]Station, error) {
	endpoint := c.StationsEndpoint
	if endpoint == "" {
		endpoint = DefaultStationsEndpoint
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Requested-With", "XMLHttpRequest")
	req.Header.Set("Referer", "https://kci.id/perjalanan-krl/jadwal-kereta")
	resp, err := c.httpClient().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %s", resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 5<<20))
	if err != nil {
		return nil, err
	}
	var result stationResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("decode JSON: %w", err)
	}
	if result.Status != http.StatusOK {
		return nil, fmt.Errorf("API status %d", result.Status)
	}
	stations := make([]Station, 0, len(result.Data))
	for _, item := range result.Data {
		if item.Enabled != 1 || strings.TrimSpace(item.ID) == "" || strings.TrimSpace(item.Name) == "" {
			continue
		}
		stations = append(stations, Station{Code: item.ID, Name: displayName(item.Name), Line: "KAI Commuter"})
	}
	sort.Slice(stations, func(i, j int) bool { return stations[i].Code < stations[j].Code })
	if len(stations) == 0 {
		return nil, fmt.Errorf("KCI returned no enabled stations")
	}
	return stations, nil
}

func (c Client) Fetch(ctx context.Context, stations []Station) (Snapshot, error) {
	if len(stations) == 0 {
		return Snapshot{}, fmt.Errorf("station catalog is empty")
	}
	if c.Endpoint == "" {
		c.Endpoint = DefaultEndpoint
	}
	if c.Retries < 1 {
		c.Retries = 3
	}
	if c.Sleep == nil {
		c.Sleep = time.Sleep
	}
	snapshot := c.Initial
	if snapshot.FetchedAt.IsZero() {
		snapshot.FetchedAt = time.Now().UTC()
	}
	if snapshot.Stations == nil {
		snapshot.Stations = make(map[string][]APIRecord, len(stations))
	}
	for i, station := range stations {
		if c.Progress != nil {
			c.Progress(i+1, len(stations), station)
		}
		if _, done := snapshot.Stations[station.Code]; done || contains(snapshot.Unsupported, station.Code) {
			continue
		}
		if i > 0 && c.Delay > 0 {
			c.Sleep(c.Delay)
		}
		records, err := c.fetchStation(ctx, station.Code)
		if errors.Is(err, ErrUnsupportedStation) {
			snapshot.Unsupported = append(snapshot.Unsupported, station.Code)
			if err := c.save(snapshot); err != nil {
				return snapshot, err
			}
			continue
		}
		if err != nil {
			return snapshot, fmt.Errorf("fetch station %s: %w", station.Code, err)
		}
		if len(records) == 0 {
			return snapshot, fmt.Errorf("fetch station %s: response has no departures", station.Code)
		}
		snapshot.Stations[station.Code] = records
		if err := c.save(snapshot); err != nil {
			return snapshot, err
		}
	}
	return snapshot, nil
}

func (c Client) save(snapshot Snapshot) error {
	if c.SaveProgress == nil {
		return nil
	}
	return c.SaveProgress(snapshot)
}

func contains(items []string, value string) bool {
	for _, item := range items {
		if item == value {
			return true
		}
	}
	return false
}

func (c Client) fetchStation(ctx context.Context, stationCode string) ([]APIRecord, error) {
	var lastErr error
	for attempt := 0; attempt < c.Retries; attempt++ {
		if attempt > 0 {
			c.Sleep(time.Duration(attempt) * 500 * time.Millisecond)
		}
		u, err := url.Parse(c.Endpoint)
		if err != nil {
			return nil, err
		}
		q := u.Query()
		q.Set("stationid", stationCode)
		q.Set("timefrom", "00:00")
		q.Set("timeto", "23:00")
		u.RawQuery = q.Encode()
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Accept", "application/json")
		req.Header.Set("X-Requested-With", "XMLHttpRequest")
		req.Header.Set("Referer", "https://kci.id/perjalanan-krl/jadwal-kereta")
		resp, err := c.httpClient().Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		body, readErr := io.ReadAll(io.LimitReader(resp.Body, 5<<20))
		resp.Body.Close()
		if readErr != nil {
			lastErr = readErr
			continue
		}
		if resp.StatusCode != http.StatusOK {
			if resp.StatusCode == http.StatusNotFound {
				return nil, ErrUnsupportedStation
			}
			lastErr = fmt.Errorf("HTTP %s", resp.Status)
			continue
		}
		var result apiResponse
		if err := json.Unmarshal(body, &result); err != nil {
			lastErr = fmt.Errorf("decode JSON: %w", err)
			continue
		}
		if result.Status != http.StatusOK {
			lastErr = fmt.Errorf("API status %d", result.Status)
			continue
		}
		return result.Data, nil
	}
	return nil, lastErr
}

func (c Client) httpClient() *http.Client {
	if c.HTTP != nil {
		return c.HTTP
	}
	return &http.Client{Timeout: 30 * time.Second}
}

type Stop struct {
	StationCode string
	Time        string
}

type Train struct {
	Number string
	Route  string
	Stops  []Stop
}

// Normalize reconstructs journeys by joining a train's departure records across stations.
func Normalize(snapshot Snapshot, stations []Station) ([]Train, error) {
	stationByName := make(map[string]string, len(stations))
	stationCodes := make(map[string]bool, len(stations))
	for _, station := range stations {
		stationByName[stationKey(station.Name)] = station.Code
		stationCodes[station.Code] = true
	}
	type aggregate struct {
		Train
		destination string
		destTime    string
		stops       map[string]string
	}
	trains := map[string]*aggregate{}
	for stationCode, records := range snapshot.Stations {
		if !stationCodes[stationCode] {
			return nil, fmt.Errorf("snapshot contains unknown station %q", stationCode)
		}
		for _, record := range records {
			if strings.TrimSpace(record.TrainID) == "" || !validTime(record.Time) || !validTime(record.DestTime) {
				return nil, fmt.Errorf("invalid record at station %s", stationCode)
			}
			item := trains[record.TrainID]
			if item == nil {
				item = &aggregate{Train: Train{Number: record.TrainID, Route: displayRoute(record.RouteName)}, destination: record.Dest, destTime: record.DestTime, stops: map[string]string{}}
				trains[record.TrainID] = item
			} else if item.Route != displayRoute(record.RouteName) || stationKey(item.destination) != stationKey(record.Dest) || item.destTime != record.DestTime {
				return nil, fmt.Errorf("conflicting records for train %s", record.TrainID)
			}
			if previous, exists := item.stops[stationCode]; exists && previous != record.Time {
				return nil, fmt.Errorf("conflicting departure times for train %s at %s", record.TrainID, stationCode)
			}
			item.stops[stationCode] = record.Time
		}
	}
	result := make([]Train, 0, len(trains))
	for _, item := range trains {
		destination, ok := destinationCode(item.destination, stationByName)
		if !ok {
			return nil, fmt.Errorf("train %s has unmapped destination %q", item.Number, item.destination)
		}
		// The station feed can expose a terminal's platform time before the
		// route's published destination time. The latter is authoritative for
		// the final arrival and avoids representing the terminal twice.
		item.stops[destination] = item.destTime
		for code, departure := range item.stops {
			item.Stops = append(item.Stops, Stop{StationCode: code, Time: clock(departure)})
		}
		item.Stops = chronological(item.Stops)
		if len(item.Stops) < 2 {
			return nil, fmt.Errorf("train %s has fewer than two stops", item.Number)
		}
		result = append(result, item.Train)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Number < result[j].Number })
	if len(result) == 0 {
		return nil, fmt.Errorf("snapshot contains no valid train journeys")
	}
	return result, nil
}

// AddDestinationStations adds KCI API terminal destinations that are absent from
// its station-picker endpoint (for example, intercity commuter extensions).
func AddDestinationStations(snapshot Snapshot, stations []Station) ([]Station, []Station) {
	known := make(map[string]bool, len(stations))
	usedCodes := make(map[string]bool, len(stations))
	for _, station := range stations {
		known[stationKey(station.Name)] = true
		usedCodes[station.Code] = true
	}
	var additions []Station
	for _, records := range snapshot.Stations {
		for _, record := range records {
			name := strings.TrimSpace(strings.Split(strings.ToLower(record.Dest), " via ")[0])
			key := stationKey(name)
			if alias, ok := destinationAliases[key]; ok {
				key = alias
			}
			if key == "" || known[key] {
				continue
			}
			code := "KCI_DEST_" + strings.ToUpper(key)
			if usedCodes[code] {
				continue
			}
			station := Station{Code: code, Name: displayName(name), Line: "KAI Commuter (destination only)"}
			stations = append(stations, station)
			additions = append(additions, station)
			known[key] = true
			usedCodes[code] = true
		}
	}
	sort.Slice(stations, func(i, j int) bool { return stations[i].Code < stations[j].Code })
	sort.Slice(additions, func(i, j int) bool { return additions[i].Name < additions[j].Name })
	return stations, additions
}

func destinationCode(destination string, stationByName map[string]string) (string, bool) {
	key := stationKey(strings.Split(strings.ToLower(destination), " via ")[0])
	if alias, ok := destinationAliases[key]; ok {
		key = alias
	}
	code, ok := stationByName[key]
	return code, ok
}

// destinationAliases documents spelling variants emitted by the public API.
var destinationAliases = map[string]string{
	"tanjungpriuk": "tanjungpriok",
}

func stationKey(value string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(value) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
		}
	}
	return b.String()
}

func displayRoute(route string) string {
	parts := strings.Split(route, "-")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return strings.Join(parts, " → ")
}

func displayName(value string) string {
	words := strings.Fields(strings.ToLower(strings.TrimSpace(value)))
	for i, word := range words {
		words[i] = strings.ToUpper(word[:1]) + word[1:]
	}
	name := strings.Join(words, " ")
	if name == "Bni City" {
		return "BNI City"
	}
	if name == "Ui" {
		return "UI"
	}
	return name
}

func validTime(value string) bool {
	_, err := time.Parse("15:04:05", value)
	return err == nil
}

func clock(value string) string { return strings.TrimSuffix(value, ":00") }

func chronological(stops []Stop) []Stop {
	sort.Slice(stops, func(i, j int) bool { return stops[i].Time < stops[j].Time })
	if len(stops) < 2 {
		return stops
	}
	start, largestGap := 0, -1
	for i := range stops {
		current := minutes(stops[i].Time)
		next := minutes(stops[(i+1)%len(stops)].Time)
		if i == len(stops)-1 {
			next += 24 * 60
		}
		if gap := next - current; gap > largestGap {
			largestGap, start = gap, (i+1)%len(stops)
		}
	}
	return append(append([]Stop{}, stops[start:]...), stops[:start]...)
}

func minutes(value string) int {
	var hours, mins int
	fmt.Sscanf(value, "%d:%d", &hours, &mins)
	return hours*60 + mins
}
