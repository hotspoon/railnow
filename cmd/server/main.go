package main

import (
	"database/sql"
	"github.com/go-chi/chi/v5"
	"github.com/hotspoon/railnow/internal/handler"
	"github.com/hotspoon/railnow/internal/repository"
	"github.com/hotspoon/railnow/internal/service"
	"github.com/pressly/goose/v3"
	"log"
	_ "modernc.org/sqlite"
	"net/http"
	"os"
	"path/filepath"
)

func main() {
	dbPath := os.Getenv("DATABASE_URL")
	if dbPath == "" {
		dbPath = "data/railnow.db"
	}
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		log.Fatal(err)
	}
	db, err := sql.Open("sqlite", dbPath)
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
	if err := seed(db); err != nil {
		log.Fatal(err)
	}
	s := service.New(repository.New(db))
	h := handler.New(s)
	r := chi.NewRouter()
	r.Get("/", h.Home)
	r.Get("/search", h.Search)
	r.Get("/schedule", h.Schedule)
	r.Get("/train/{id}", h.Train)
	r.Post("/favorite", h.Favorite)
	r.Handle("/public/*", http.StripPrefix("/public/", http.FileServer(http.Dir("public"))))
	r.Get("/manifest.webmanifest", func(w http.ResponseWriter, r *http.Request) { http.ServeFile(w, r, "public/manifest.webmanifest") })
	r.Get("/sw.js", func(w http.ResponseWriter, r *http.Request) { http.ServeFile(w, r, "public/sw.js") })
	addr := os.Getenv("PORT")
	if addr == "" {
		addr = "8080"
	}
	log.Printf("RailNow is running on http://localhost:%s", addr)
	log.Fatal(http.ListenAndServe(":"+addr, r))
}
func seed(db *sql.DB) error {
	var n int
	if err := db.QueryRow("SELECT COUNT(*) FROM stations").Scan(&n); err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	b, err := os.ReadFile("data/seed.sql")
	if err != nil {
		return err
	}
	_, err = db.Exec(string(b))
	return err
}
