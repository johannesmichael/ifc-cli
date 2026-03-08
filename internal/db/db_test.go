package db

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOpenInMemory(t *testing.T) {
	d, err := Open("")
	if err != nil {
		t.Fatalf("Open in-memory: %v", err)
	}
	defer d.Close()

	if d.DB == nil {
		t.Fatal("expected non-nil DB")
	}
}

func TestSchemaTablesExist(t *testing.T) {
	d, err := Open("")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer d.Close()

	tables := []string{
		"entities",
		"properties",
		"quantities",
		"relationships",
		"spatial_structure",
		"geometry",
		"file_metadata",
		"properties_v",
		"quantities_v",
		"relationships_v",
		"spatial_structure_v",
		"geometry_v",
	}

	for _, table := range tables {
		var count int
		err := d.DB.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&count)
		if err != nil {
			t.Errorf("table %s: %v", table, err)
		}
	}
}

func TestOpenFileBased(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.duckdb")

	d, err := Open(path)
	if err != nil {
		t.Fatalf("Open file: %v", err)
	}
	d.Close()

	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("expected database file to exist")
	}
}

func TestOpenIdempotent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.duckdb")

	// First open creates schema
	d1, err := Open(path)
	if err != nil {
		t.Fatalf("first Open: %v", err)
	}

	// Insert a row to verify data persists
	_, err = d1.DB.Exec("INSERT INTO file_metadata VALUES ('test_key', 'test_value')")
	if err != nil {
		t.Fatalf("insert: %v", err)
	}
	d1.Close()

	// Second open should succeed (CREATE IF NOT EXISTS)
	d2, err := Open(path)
	if err != nil {
		t.Fatalf("second Open: %v", err)
	}
	defer d2.Close()

	var val string
	err = d2.DB.QueryRow("SELECT value FROM file_metadata WHERE key = 'test_key'").Scan(&val)
	if err != nil {
		t.Fatalf("query after reopen: %v", err)
	}
	if val != "test_value" {
		t.Errorf("got %q, want %q", val, "test_value")
	}
}
