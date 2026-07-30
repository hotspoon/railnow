// Command migrate-turso copies RuteKRL's local SQLite data into a Turso database.
package main

import (
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/hotspoon/railnow/internal/database"
	"github.com/pressly/goose/v3"
)

type table struct {
	name    string
	columns string
}

var tables = []table{
	{"stations", "id, code, name, line"},
	{"trains", "id, train_number, route_name"},
	{"schedules", "id, train_id, station_id, arrival, departure, sequence, arrival_estimated"},
	{"favorites", "id, from_station, to_station, label, created_at"},
	{"searches", "id, from_station, to_station, created_at"},
	{"schedule_metadata", "key, value"},
}

const insertBatchSize = 100

func main() {
	sourcePath := flag.String("source", "data/railnow.db", "source SQLite database path")
	replace := flag.Bool("replace", false, "replace data already in the Turso database")
	flag.Parse()

	remoteURL := os.Getenv("TURSO_DATABASE_URL")
	if remoteURL == "" {
		log.Fatal("TURSO_DATABASE_URL is required")
	}
	source, err := database.OpenSQLite(*sourcePath)
	if err != nil {
		log.Fatal(err)
	}
	defer source.Close()
	target, err := database.OpenTurso(remoteURL, os.Getenv("TURSO_AUTH_TOKEN"))
	if err != nil {
		log.Fatal(err)
	}
	defer target.Close()
	if err := goose.SetDialect("sqlite3"); err != nil {
		log.Fatal(err)
	}
	if err := goose.Up(target, "db/migrations"); err != nil {
		log.Fatalf("apply Turso migrations: %v", err)
	}
	if err := copyDatabase(source, target, *replace); err != nil {
		log.Fatal(err)
	}
	log.Printf("Migrated %s to Turso", *sourcePath)
}

func copyDatabase(source, target *sql.DB, replace bool) error {
	var count int
	if err := target.QueryRow("SELECT COUNT(*) FROM stations").Scan(&count); err != nil {
		return fmt.Errorf("check Turso data: %w", err)
	}
	if count > 0 && !replace {
		return errors.New("Turso database already has data; rerun with --replace to overwrite it")
	}
	tx, err := target.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if replace {
		for _, name := range []string{"searches", "favorites", "schedules", "trains", "stations", "schedule_metadata"} {
			if _, err := tx.Exec("DELETE FROM " + name); err != nil {
				return fmt.Errorf("clear %s: %w", name, err)
			}
		}
	}
	for _, item := range tables {
		if err := copyTable(source, tx, item); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func copyTable(source *sql.DB, target *sql.Tx, item table) error {
	rows, err := source.Query("SELECT " + item.columns + " FROM " + item.name)
	if err != nil {
		return fmt.Errorf("read %s: %w", item.name, err)
	}
	defer rows.Close()
	columnNames, err := rows.Columns()
	if err != nil {
		return err
	}
	values := make([][]any, 0, insertBatchSize)
	inserted := 0
	for rows.Next() {
		rowValues := make([]any, len(columnNames))
		pointers := make([]any, len(columnNames))
		for i := range rowValues {
			pointers[i] = &rowValues[i]
		}
		if err := rows.Scan(pointers...); err != nil {
			return fmt.Errorf("scan %s: %w", item.name, err)
		}
		values = append(values, rowValues)
		if len(values) == insertBatchSize {
			if err := insertRows(target, item, columnNames, values); err != nil {
				return err
			}
			inserted += len(values)
			log.Printf("Copied %d %s rows", inserted, item.name)
			values = values[:0]
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if len(values) > 0 {
		if err := insertRows(target, item, columnNames, values); err != nil {
			return err
		}
		inserted += len(values)
	}
	log.Printf("Copied %d %s rows", inserted, item.name)
	return nil
}

func insertRows(target *sql.Tx, item table, columns []string, rows [][]any) error {
	rowPlaceholders := "(" + strings.TrimRight(strings.Repeat("?,", len(columns)), ",") + ")"
	placeholders := strings.TrimRight(strings.Repeat(rowPlaceholders+",", len(rows)), ",")
	args := make([]any, 0, len(rows)*len(columns))
	for _, row := range rows {
		args = append(args, row...)
	}
	statement := "INSERT INTO " + item.name + " (" + item.columns + ") VALUES " + placeholders
	if _, err := target.Exec(statement, args...); err != nil {
		return fmt.Errorf("write %s: %w", item.name, err)
	}
	return nil
}
