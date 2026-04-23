package harness

import (
	"database/sql"
	"fmt"
	"os"
	"sync"

	_ "github.com/lib/pq"
)

const defaultDBURL = "postgres://lustia_app:lustia_app_local@localhost:5432/lustia?sslmode=disable"

var (
	dbOnce sync.Once
	dbConn *sql.DB
	dbErr  error
)

// DB returns a shared *sql.DB for the test suite.
// The connection URL is taken from the TEST_DB_URL environment variable,
// falling back to the local dev default.
func DB() (*sql.DB, error) {
	dbOnce.Do(func() {
		url := os.Getenv("TEST_DB_URL")
		if url == "" {
			url = defaultDBURL
		}
		dbConn, dbErr = sql.Open("postgres", url)
		if dbErr != nil {
			return
		}
		dbErr = dbConn.Ping()
	})
	return dbConn, dbErr
}

// ScanOne executes query with args and scans the single result row into dest.
// Returns sql.ErrNoRows if no row was found.
func ScanOne[T any](query string, dest func(*sql.Row) (T, error), args ...any) (T, error) {
	db, err := DB()
	if err != nil {
		var zero T
		return zero, fmt.Errorf("db connection: %w", err)
	}
	row := db.QueryRow(query, args...)
	return dest(row)
}

// Exec runs a write query against the DB (useful for teardown / seeding in tests).
func Exec(query string, args ...any) error {
	db, err := DB()
	if err != nil {
		return fmt.Errorf("db connection: %w", err)
	}
	_, err = db.Exec(query, args...)
	return err
}

// QueryRows executes a query and returns all rows scanned via scanFn.
func QueryRows[T any](query string, scanFn func(*sql.Rows) (T, error), args ...any) ([]T, error) {
	db, err := DB()
	if err != nil {
		return nil, fmt.Errorf("db connection: %w", err)
	}
	rows, err := db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var results []T
	for rows.Next() {
		val, err := scanFn(rows)
		if err != nil {
			return nil, err
		}
		results = append(results, val)
	}
	return results, rows.Err()
}
