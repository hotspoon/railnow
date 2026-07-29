package kci

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestClientFetchesEveryStation(t *testing.T) {
	var requested []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requested = append(requested, r.URL.Query().Get("stationid"))
		if got := r.URL.Query().Get("timefrom"); got != "00:00" {
			t.Fatalf("timefrom = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":200,"data":[{"train_id":"1","route_name":"A-B","dest":"B","time_est":"08:00:00","dest_time":"08:10:00"}]}`))
	}))
	defer server.Close()
	client := Client{Endpoint: server.URL, HTTP: server.Client(), Delay: time.Millisecond, Sleep: func(time.Duration) {}}
	snapshot, err := client.Fetch(context.Background(), []Station{{Code: "A"}, {Code: "B"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Stations) != 2 || strings.Join(requested, ",") != "A,B" {
		t.Fatalf("stations = %#v, requests = %v", snapshot.Stations, requested)
	}
}

func TestFetchStationsUsesEnabledKCIStations(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":200,"data":[{"sta_id":"WIL0","sta_name":"AREA JABODETABEK","fg_enable":0},{"sta_id":"BUA","sta_name":"BUARAN","fg_enable":1},{"sta_id":"CKR","sta_name":"CIKARANG","fg_enable":1}]}`))
	}))
	defer server.Close()
	stations, err := (Client{StationsEndpoint: server.URL, HTTP: server.Client()}).FetchStations(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(stations) != 2 || stations[0].Code != "BUA" || stations[0].Name != "Buaran" {
		t.Fatalf("stations = %#v", stations)
	}
}

func TestClientSkipsUnsupportedStationWithoutRetry(t *testing.T) {
	requests := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		http.NotFound(w, r)
	}))
	defer server.Close()
	snapshot, err := (Client{Endpoint: server.URL, HTTP: server.Client(), Retries: 3}).Fetch(context.Background(), []Station{{Code: "GGL"}})
	if err != nil {
		t.Fatal(err)
	}
	if requests != 1 || len(snapshot.Unsupported) != 1 || snapshot.Unsupported[0] != "GGL" {
		t.Fatalf("requests=%d snapshot=%#v", requests, snapshot)
	}
}

func TestClientResumesSavedStations(t *testing.T) {
	var requested []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requested = append(requested, r.URL.Query().Get("stationid"))
		w.Write([]byte(`{"status":200,"data":[{"train_id":"1","route_name":"A-B","dest":"B","time_est":"08:00:00","dest_time":"08:10:00"}]}`))
	}))
	defer server.Close()
	client := Client{Endpoint: server.URL, HTTP: server.Client(), Initial: Snapshot{FetchedAt: time.Now(), Stations: map[string][]APIRecord{"A": []APIRecord{{TrainID: "old"}}}}}
	snapshot, err := client.Fetch(context.Background(), []Station{{Code: "A"}, {Code: "B"}})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(requested, ",") != "B" || len(snapshot.Stations) != 2 {
		t.Fatalf("requests=%v snapshot=%#v", requested, snapshot)
	}
}

func TestNormalizeOrdersOvernightAndAddsTerminal(t *testing.T) {
	stations := []Station{{Code: "BUA", Name: "Buaran"}, {Code: "CKR", Name: "Cikarang"}}
	snapshot := Snapshot{Stations: map[string][]APIRecord{
		"BUA": {{TrainID: "5192B", RouteName: "BUARAN-CIKARANG", Dest: "CIKARANG VIA MRI", Time: "23:50:00", DestTime: "00:30:00"}},
	}}
	trains, err := Normalize(snapshot, stations)
	if err != nil {
		t.Fatal(err)
	}
	if len(trains) != 1 || len(trains[0].Stops) != 2 {
		t.Fatalf("trains = %#v", trains)
	}
	if got := trains[0].Stops; got[0].StationCode != "BUA" || got[1].StationCode != "CKR" || got[1].Time != "00:30" {
		t.Fatalf("stops = %#v", got)
	}
}

func TestNormalizeMapsKCIStationSpellingVariant(t *testing.T) {
	stations := []Station{{Code: "JAK", Name: "Tanjung Priok"}, {Code: "AC", Name: "Ancol"}}
	trains, err := Normalize(Snapshot{Stations: map[string][]APIRecord{
		"AC": {{TrainID: "2260", RouteName: "ANCOL-TANJUNGPRIUK", Dest: "TANJUNGPRIUK", Time: "08:00:00", DestTime: "08:10:00"}},
	}}, stations)
	if err != nil {
		t.Fatal(err)
	}
	if got := trains[0].Stops[1].StationCode; got != "JAK" {
		t.Fatalf("terminal = %q", got)
	}
}

func TestNormalizeUsesDestinationTimeForExistingTerminalStop(t *testing.T) {
	stations := []Station{{Code: "CKR", Name: "Cikarang"}, {Code: "AK", Name: "Angke"}}
	trains, err := Normalize(Snapshot{Stations: map[string][]APIRecord{
		"CKR": {{TrainID: "5095B", RouteName: "CIKARANG-ANGKE", Dest: "ANGKE", Time: "11:04:00", DestTime: "12:28:00"}},
		"AK":  {{TrainID: "5095B", RouteName: "CIKARANG-ANGKE", Dest: "ANGKE", Time: "12:20:00", DestTime: "12:28:00"}},
	}}, stations)
	if err != nil {
		t.Fatal(err)
	}
	if got := trains[0].Stops[1].Time; got != "12:28" {
		t.Fatalf("terminal time = %q", got)
	}
}

func TestAddDestinationStationsIncludesUnlistedTerminal(t *testing.T) {
	stations, additions := AddDestinationStations(Snapshot{Stations: map[string][]APIRecord{
		"AC": {{TrainID: "326", Dest: "PURWAKARTA", Time: "08:00:00", DestTime: "09:00:00"}},
	}}, []Station{{Code: "AC", Name: "Ancol"}})
	if len(additions) != 1 || additions[0].Code != "KCI_DEST_PURWAKARTA" || additions[0].Name != "Purwakarta" {
		t.Fatalf("additions = %#v", additions)
	}
	if len(stations) != 2 {
		t.Fatalf("stations = %#v", stations)
	}
}

func TestNormalizeRejectsConflictingTrain(t *testing.T) {
	stations := []Station{{Code: "A", Name: "Alpha"}, {Code: "B", Name: "Beta"}}
	_, err := Normalize(Snapshot{Stations: map[string][]APIRecord{
		"A": {{TrainID: "1", RouteName: "A-B", Dest: "Beta", Time: "08:00:00", DestTime: "08:10:00"}},
		"B": {{TrainID: "1", RouteName: "A-C", Dest: "Beta", Time: "08:10:00", DestTime: "08:10:00"}},
	}}, stations)
	if err == nil || !strings.Contains(err.Error(), "conflicting") {
		t.Fatalf("err = %v", err)
	}
}
