package db

import (
	"database/sql"

	_ "github.com/marcboeker/go-duckdb"
)

// Database wraps a DuckDB connection.
type Database struct {
	DB   *sql.DB
	Path string
}

// Open creates or opens a DuckDB database at path.
// An empty path creates an in-memory database.
func Open(path string) (*Database, error) {
	db, err := sql.Open("duckdb", path)
	if err != nil {
		return nil, err
	}
	if err := CreateSchema(db); err != nil {
		db.Close()
		return nil, err
	}
	return &Database{DB: db, Path: path}, nil
}

// Close closes the underlying database connection.
func (d *Database) Close() error {
	return d.DB.Close()
}
