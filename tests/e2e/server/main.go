package main

import (
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/hotspoon/railnow/internal/app"
	"github.com/hotspoon/railnow/internal/database"
	"github.com/pressly/goose/v3"
)

func main() {
	databasePath := filepath.Join(os.TempDir(), "railnow-playwright.db")
	if err := os.Remove(databasePath); err != nil && !os.IsNotExist(err) {
		log.Fatal(err)
	}
	db, err := database.OpenSQLite(databasePath)
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
	seed, err := os.ReadFile("data/seed.sql")
	if err != nil {
		log.Fatal(err)
	}
	if _, err := db.Exec(string(seed)); err != nil {
		log.Fatal(err)
	}
	metadata := map[string]string{
		"snapshot_date":  time.Now().Format("02 Jan 2006"),
		"effective_date": "Playwright fixture",
		"source_name":    "RailNow deterministic E2E fixture",
		"source_url":     "https://example.test/railnow-fixture",
		"fetched_at":     time.Now().UTC().Format(time.RFC3339),
		"day_type":       "Test schedule",
	}
	for key, value := range metadata {
		if _, err := db.Exec(`INSERT INTO schedule_metadata(key,value) VALUES(?,?)`, key, value); err != nil {
			log.Fatal(err)
		}
	}

	router := app.NewHandler(db)
	router.Handle("/css/*", http.StripPrefix("/", http.FileServer(http.Dir("public"))))
	router.Handle("/js/*", http.StripPrefix("/", http.FileServer(http.Dir("public"))))
	router.Handle("/icons/*", http.StripPrefix("/", http.FileServer(http.Dir("public"))))
	router.Handle("/images/*", http.StripPrefix("/", http.FileServer(http.Dir("public"))))
	log.Fatal(http.ListenAndServe("127.0.0.1:4173", router))
}
