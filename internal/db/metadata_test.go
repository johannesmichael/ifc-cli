package db

import (
	"testing"

	"ifc-cli/internal/step"
)

func TestWriteMetadata(t *testing.T) {
	d, err := Open("")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer d.Close()

	meta := &step.FileMetadata{
		Description:       "ViewDefinition [CoordinationView]",
		ImplementationLevel: "2;1",
		FileName:          "test.ifc",
		Timestamp:         "2024-01-15T10:30:00",
		Author:            []string{"John Doe", "Jane Smith"},
		Organization:      []string{"ACME Corp"},
		Preprocessor:      "PreprocessorX",
		OriginatingSystem: "TestApp",
		Authorization:     "Auth123",
		SchemaIdentifiers: []string{"IFC4"},
	}

	extra := map[string]string{
		"entity_count":      "500",
		"parse_duration_ms": "42",
	}

	if err := WriteMetadata(d.DB, meta, extra); err != nil {
		t.Fatalf("WriteMetadata: %v", err)
	}

	expected := map[string]string{
		"description":        "ViewDefinition [CoordinationView]",
		"implementation_level": "2;1",
		"file_name":          "test.ifc",
		"timestamp":          "2024-01-15T10:30:00",
		"author":             "John Doe; Jane Smith",
		"organization":       "ACME Corp",
		"preprocessor":       "PreprocessorX",
		"originating_system": "TestApp",
		"authorization":      "Auth123",
		"schema_identifiers": "IFC4",
		"entity_count":       "500",
		"parse_duration_ms":  "42",
	}

	for key, want := range expected {
		var got string
		err := d.DB.QueryRow("SELECT value FROM file_metadata WHERE key = ?", key).Scan(&got)
		if err != nil {
			t.Errorf("key %q: %v", key, err)
			continue
		}
		if got != want {
			t.Errorf("key %q: got %q, want %q", key, got, want)
		}
	}
}

func TestWriteMetadataNilMeta(t *testing.T) {
	d, err := Open("")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer d.Close()

	extra := map[string]string{
		"source_file_size": "1024",
	}

	if err := WriteMetadata(d.DB, nil, extra); err != nil {
		t.Fatalf("WriteMetadata: %v", err)
	}

	var got string
	err = d.DB.QueryRow("SELECT value FROM file_metadata WHERE key = 'source_file_size'").Scan(&got)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if got != "1024" {
		t.Errorf("got %q, want %q", got, "1024")
	}
}

func TestWriteMetadataSkipsEmpty(t *testing.T) {
	d, err := Open("")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer d.Close()

	meta := &step.FileMetadata{
		FileName: "test.ifc",
		// All other fields empty
	}

	if err := WriteMetadata(d.DB, meta, nil); err != nil {
		t.Fatalf("WriteMetadata: %v", err)
	}

	var count int
	err = d.DB.QueryRow("SELECT COUNT(*) FROM file_metadata").Scan(&count)
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 row (file_name only), got %d", count)
	}
}
