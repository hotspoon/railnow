// Package database opens the application's local SQLite or remote Turso database.
package database

import (
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	_ "github.com/tursodatabase/libsql-client-go/libsql"
	_ "modernc.org/sqlite"
)

// OpenFromEnv prefers Turso when TURSO_DATABASE_URL is set. DATABASE_URL remains
// a path to a local SQLite database for development.
func OpenFromEnv(localPath string) (*sql.DB, error) {
	if remoteURL := os.Getenv("TURSO_DATABASE_URL"); remoteURL != "" {
		return OpenTurso(remoteURL, os.Getenv("TURSO_AUTH_TOKEN"))
	}
	if path := os.Getenv("DATABASE_URL"); path != "" {
		localPath = path
	}
	return OpenSQLite(localPath)
}

func OpenSQLite(path string) (*sql.DB, error) {
	if path != ":memory:" {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return nil, fmt.Errorf("create SQLite directory: %w", err)
		}
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open SQLite database: %w", err)
	}
	return db, nil
}

func OpenTurso(databaseURL, authToken string) (*sql.DB, error) {
	if authToken == "" {
		return nil, errors.New("TURSO_AUTH_TOKEN is required when TURSO_DATABASE_URL is set")
	}
	u, err := url.Parse(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse TURSO_DATABASE_URL: %w", err)
	}
	if u.Scheme != "libsql" && u.Scheme != "https" {
		return nil, fmt.Errorf("TURSO_DATABASE_URL must use libsql:// or https://, got %q", u.Scheme)
	}
	query := u.Query()
	query.Set("authToken", authToken)
	u.RawQuery = query.Encode()
	db, err := sql.Open("libsql", u.String())
	if err != nil {
		return nil, fmt.Errorf("open Turso database: %w", err)
	}
	return db, nil
}

// IsRemoteURL reports whether a database string should be opened through Turso.
func IsRemoteURL(databaseURL string) bool {
	return strings.HasPrefix(databaseURL, "libsql://") || strings.HasPrefix(databaseURL, "https://")
}
