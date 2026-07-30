// Package platform provides hosting-specific entry points for RuteKRL.
package platform

import (
	"database/sql"
	"errors"
	"log"
	"net/http"
	"os"
	"sync"

	"github.com/hotspoon/railnow/internal/app"
	"github.com/hotspoon/railnow/internal/database"
)

var (
	once        sync.Once
	application http.Handler
	initErr     error
)

// VercelHandler initializes the Turso-backed application once per Function
// instance, then serves the incoming request.
func VercelHandler(w http.ResponseWriter, r *http.Request) {
	once.Do(initialize)
	if initErr != nil {
		log.Printf("initialize application: %v", initErr)
		http.Error(w, "Service temporarily unavailable", http.StatusServiceUnavailable)
		return
	}
	application.ServeHTTP(w, r)
}

func initialize() {
	if os.Getenv("TURSO_DATABASE_URL") == "" {
		initErr = errors.New("TURSO_DATABASE_URL is required on Vercel")
		return
	}
	var db *sql.DB
	db, initErr = database.OpenFromEnv("/tmp/railnow.db")
	if initErr != nil {
		return
	}
	if initErr = db.Ping(); initErr != nil {
		db.Close()
		return
	}
	application = app.NewHandler(db)
}
