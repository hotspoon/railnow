// Command scraper downloads published KAI Commuter timetable CSV files.
// It never modifies a database; import is a separate, atomic step.
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const defaultPage = "https://www.commuterline.id/layanan/info-pelanggan/jadwal"

func main() {
	page := flag.String("page", defaultPage, "KAI Commuter schedule publication URL")
	scheduleURL := flag.String("schedule-url", os.Getenv("KAI_SCHEDULE_CSV_URL"), "published schedule CSV URL")
	stationsURL := flag.String("stations-url", os.Getenv("KAI_STATIONS_CSV_URL"), "published station CSV URL")
	out := flag.String("out", ".cache/kai", "download directory")
	flag.Parse()
	if *scheduleURL == "" {
		var err error
		*scheduleURL, err = discoverCSV(*page, "schedule")
		if err != nil {
			log.Fatal(fmt.Errorf("discover schedule CSV: %w; provide --schedule-url", err))
		}
	}
	if *stationsURL == "" {
		var err error
		*stationsURL, err = discoverCSV(*page, "station")
		if err != nil {
			log.Fatal(fmt.Errorf("discover station CSV: %w; provide --stations-url", err))
		}
	}
	if err := os.MkdirAll(*out, 0o755); err != nil {
		log.Fatal(err)
	}
	if err := download(*scheduleURL, filepath.Join(*out, "commuter_line_schedule.csv")); err != nil {
		log.Fatal(err)
	}
	if err := download(*stationsURL, filepath.Join(*out, "dim_station.csv")); err != nil {
		log.Fatal(err)
	}
}

func discoverCSV(pageURL, keyword string) (string, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(pageURL)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("schedule page returned %s", resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 5<<20))
	if err != nil {
		return "", err
	}
	re := regexp.MustCompile(`(?i)(?:href|src)=["']([^"']+\.csv[^"']*)`)
	base, _ := url.Parse(pageURL)
	for _, match := range re.FindAllStringSubmatch(string(body), -1) {
		if strings.Contains(strings.ToLower(match[1]), keyword) {
			u, err := base.Parse(match[1])
			if err == nil {
				return u.String(), nil
			}
		}
	}
	return "", errors.New("no matching CSV link found in the published page")
}

func download(source, target string) error {
	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Get(source)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download %s: %s", source, resp.Status)
	}
	f, err := os.Create(target)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, io.LimitReader(resp.Body, 100<<20))
	return err
}
