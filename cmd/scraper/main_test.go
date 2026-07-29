package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDiscoverCSV(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`<a href="/files/commuter_schedule.csv">schedule</a><a href="/files/dim_station.csv">stations</a>`))
	}))
	defer server.Close()
	got, err := discoverCSV(server.URL, "schedule")
	if err != nil || got != server.URL+"/files/commuter_schedule.csv" {
		t.Fatalf("discoverCSV() = %q, %v", got, err)
	}
}
