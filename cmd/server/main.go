package main

import (
	"database/sql"
	"log"
	"net/http"
	"os"

	"github.com/hotspoon/railnow/internal/app"
	"github.com/hotspoon/railnow/internal/database"
	"github.com/pressly/goose/v3"
)

func main() {
	db, err := database.OpenFromEnv("data/railnow.db")
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
	r := app.NewHandler(db)
	r.Handle("/css/*", http.StripPrefix("/", http.FileServer(http.Dir("public"))))
	r.Handle("/js/*", http.StripPrefix("/", http.FileServer(http.Dir("public"))))
	r.Handle("/icons/*", http.StripPrefix("/", http.FileServer(http.Dir("public"))))
	r.Handle("/images/*", http.StripPrefix("/", http.FileServer(http.Dir("public"))))
	r.Get("/favicon.ico", func(w http.ResponseWriter, r *http.Request) { http.ServeFile(w, r, "public/icons/favicon.ico") })
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
