package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"ifc-cli/internal/db"
	"ifc-cli/internal/step"
)

func TestImportMinimalIFC(t *testing.T) {
	testFile := filepath.Join("..", "step", "testdata", "minimal.ifc")
	if _, err := os.Stat(testFile); err != nil {
		t.Skipf("test fixture not found: %s", testFile)
	}

	data, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("reading test file: %v", err)
	}

	database, err := db.Open("")
	if err != nil {
		t.Fatalf("opening database: %v", err)
	}
	defer database.Close()

	parser := step.NewParser(data)
	writer, err := db.NewWriter(database, 100)
	if err != nil {
		t.Fatalf("creating writer: %v", err)
	}

	var count int
	for {
		entity, err := parser.Next()
		if err != nil {
			break
		}
		if err := writer.Write(entity); err != nil {
			t.Fatalf("writing entity: %v", err)
		}
		count++
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("closing writer: %v", err)
	}

	// minimal.ifc has 13 entities (#1 through #13)
	if count != 13 {
		t.Errorf("parsed %d entities, want 13", count)
	}

	var dbCount int
	err = database.DB.QueryRow("SELECT COUNT(*) FROM entities").Scan(&dbCount)
	if err != nil {
		t.Fatalf("querying entity count: %v", err)
	}
	if dbCount != 13 {
		t.Errorf("database has %d entities, want 13", dbCount)
	}
}

func TestImportJSONAttrsValid(t *testing.T) {
	testFile := filepath.Join("..", "step", "testdata", "wall_with_properties.ifc")
	if _, err := os.Stat(testFile); err != nil {
		t.Skipf("test fixture not found: %s", testFile)
	}

	data, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("reading test file: %v", err)
	}

	database, err := db.Open("")
	if err != nil {
		t.Fatalf("opening database: %v", err)
	}
	defer database.Close()

	parser := step.NewParser(data)
	writer, err := db.NewWriter(database, 100)
	if err != nil {
		t.Fatalf("creating writer: %v", err)
	}

	for {
		entity, err := parser.Next()
		if err != nil {
			break
		}
		if err := writer.Write(entity); err != nil {
			t.Fatalf("writing entity: %v", err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("closing writer: %v", err)
	}

	// Verify all attrs are valid JSON using DuckDB's JSON functions
	rows, err := database.DB.Query("SELECT id, ifc_type, attrs FROM entities")
	if err != nil {
		t.Fatalf("querying entities: %v", err)
	}
	defer rows.Close()

	for rows.Next() {
		var id uint32
		var ifcType, attrs string
		if err := rows.Scan(&id, &ifcType, &attrs); err != nil {
			t.Fatalf("scanning row: %v", err)
		}
		if !json.Valid([]byte(attrs)) {
			t.Errorf("entity #%d (%s) has invalid JSON attrs: %s", id, ifcType, attrs)
		}
		// Verify it's a JSON array
		var arr []json.RawMessage
		if err := json.Unmarshal([]byte(attrs), &arr); err != nil {
			t.Errorf("entity #%d (%s) attrs is not a JSON array: %v", id, ifcType, err)
		}
	}
}

func TestImportJSONAttrsWithDuckDB(t *testing.T) {
	testFile := filepath.Join("..", "step", "testdata", "minimal.ifc")
	if _, err := os.Stat(testFile); err != nil {
		t.Skipf("test fixture not found: %s", testFile)
	}

	data, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("reading test file: %v", err)
	}

	database, err := db.Open("")
	if err != nil {
		t.Fatalf("opening database: %v", err)
	}
	defer database.Close()

	parser := step.NewParser(data)
	writer, err := db.NewWriter(database, 100)
	if err != nil {
		t.Fatalf("creating writer: %v", err)
	}

	for {
		entity, err := parser.Next()
		if err != nil {
			break
		}
		if err := writer.Write(entity); err != nil {
			t.Fatalf("writing entity: %v", err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("closing writer: %v", err)
	}

	// Use DuckDB JSON functions to validate attrs
	var validCount int
	err = database.DB.QueryRow("SELECT COUNT(*) FROM entities WHERE json_valid(attrs)").Scan(&validCount)
	if err != nil {
		t.Fatalf("JSON validation query: %v", err)
	}

	var totalCount int
	err = database.DB.QueryRow("SELECT COUNT(*) FROM entities").Scan(&totalCount)
	if err != nil {
		t.Fatalf("count query: %v", err)
	}

	if validCount != totalCount {
		t.Errorf("only %d of %d entities have valid JSON attrs", validCount, totalCount)
	}

	// Test json_array_length on a known entity (IFCPROJECT #1 has 9 attrs)
	var attrs string
	err = database.DB.QueryRow("SELECT attrs FROM entities WHERE id = 1").Scan(&attrs)
	if err != nil {
		t.Fatalf("SELECT attrs query: %v", err)
	}
	var arr []json.RawMessage
	if err := json.Unmarshal([]byte(attrs), &arr); err != nil {
		t.Fatalf("IFCPROJECT attrs is not a JSON array: %v\nattrs: %s", err, attrs)
	}
	if len(arr) != 9 {
		t.Errorf("IFCPROJECT attrs array length = %d, want 9\nattrs: %s", len(arr), attrs)
	}
}

func TestWriteOutputJSON(t *testing.T) {
	result := &ImportResult{
		Status:          "ok",
		InputFile:       "test.ifc",
		OutputFile:      "test.duckdb",
		DurationMs:      42,
		EntitiesParsed:  100,
		EntitiesErrored: 2,
		TablesPopulated: []string{"entities"},
		RowCounts:       map[string]int64{"entities": 100},
	}

	var buf bytes.Buffer
	if err := WriteOutput(&buf, "json", result); err != nil {
		t.Fatalf("WriteOutput: %v", err)
	}

	// Verify it's valid JSON
	var decoded ImportResult
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("invalid JSON output: %v\n%s", err, buf.String())
	}

	if decoded.Status != "ok" {
		t.Errorf("status = %q, want %q", decoded.Status, "ok")
	}
	if decoded.EntitiesParsed != 100 {
		t.Errorf("entities_parsed = %d, want 100", decoded.EntitiesParsed)
	}
}

func TestWriteOutputText(t *testing.T) {
	result := &ImportResult{
		Status:          "ok",
		InputFile:       "test.ifc",
		OutputFile:      "test.duckdb",
		DurationMs:      42,
		EntitiesParsed:  100,
		EntitiesErrored: 0,
		TablesPopulated: []string{"entities"},
		RowCounts:       map[string]int64{"entities": 100},
	}

	var buf bytes.Buffer
	if err := WriteOutput(&buf, "text", result); err != nil {
		t.Fatalf("WriteOutput: %v", err)
	}

	output := buf.String()
	if !bytes.Contains([]byte(output), []byte("ok")) {
		t.Errorf("text output missing status, got: %s", output)
	}
	if !bytes.Contains([]byte(output), []byte("100")) {
		t.Errorf("text output missing entity count, got: %s", output)
	}
}

func TestProgressReporterQuiet(t *testing.T) {
	var buf bytes.Buffer
	p := NewProgressReporter(&buf, 1000, true, false)
	p.Update(500, 10)
	p.Finish(20, 0)

	if buf.Len() != 0 {
		t.Errorf("quiet mode should produce no output, got: %s", buf.String())
	}
}

func TestImportEndToEndQueries(t *testing.T) {
	testFile := filepath.Join("..", "step", "testdata", "minimal.ifc")
	if _, err := os.Stat(testFile); err != nil {
		t.Skipf("test fixture not found: %s", testFile)
	}

	data, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatalf("reading test file: %v", err)
	}

	database, err := db.Open("")
	if err != nil {
		t.Fatalf("opening database: %v", err)
	}
	defer database.Close()

	parser := step.NewParser(data)
	writer, err := db.NewWriter(database, 100)
	if err != nil {
		t.Fatalf("creating writer: %v", err)
	}

	for {
		entity, err := parser.Next()
		if err != nil {
			break
		}
		if err := writer.Write(entity); err != nil {
			t.Fatalf("writing entity: %v", err)
		}
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("closing writer: %v", err)
	}

	// Query: entity count should be 13
	var count int
	if err := database.DB.QueryRow("SELECT COUNT(*) FROM entities").Scan(&count); err != nil {
		t.Fatalf("count query: %v", err)
	}
	if count != 13 {
		t.Errorf("entity count = %d, want 13", count)
	}

	// Query: entity #1 should be IFCPROJECT
	var ifcType string
	if err := database.DB.QueryRow("SELECT ifc_type FROM entities WHERE id=1").Scan(&ifcType); err != nil {
		t.Fatalf("ifc_type query: %v", err)
	}
	if ifcType != "IFCPROJECT" {
		t.Errorf("entity #1 ifc_type = %q, want IFCPROJECT", ifcType)
	}
}

func TestImportJSONOutputFormat(t *testing.T) {
	result := &ImportResult{
		Status:          "ok",
		InputFile:       "test.ifc",
		OutputFile:      "test.duckdb",
		DurationMs:      123,
		EntitiesParsed:  50,
		EntitiesErrored: 0,
		TablesPopulated: []string{"entities"},
		RowCounts:       map[string]int64{"entities": 50},
	}

	var buf bytes.Buffer
	if err := WriteOutput(&buf, "json", result); err != nil {
		t.Fatalf("WriteOutput: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("invalid JSON: %v\n%s", err, buf.String())
	}

	for _, field := range []string{"status", "entities_parsed", "duration_ms"} {
		if _, ok := decoded[field]; !ok {
			t.Errorf("JSON output missing field %q", field)
		}
	}
}

func TestExitCodeConstants(t *testing.T) {
	tests := []struct {
		name string
		code int
		want int
	}{
		{"ExitSuccess", ExitSuccess, 0},
		{"ExitParseError", ExitParseError, 1},
		{"ExitFileNotFound", ExitFileNotFound, 2},
		{"ExitDatabaseError", ExitDatabaseError, 3},
		{"ExitBadArguments", ExitBadArguments, 4},
		{"ExitPartialSuccess", ExitPartialSuccess, 5},
	}
	for _, tt := range tests {
		if tt.code != tt.want {
			t.Errorf("%s = %d, want %d", tt.name, tt.code, tt.want)
		}
	}
}

func TestImportNonexistentFileError(t *testing.T) {
	cmd := importCmd
	err := cmd.RunE(cmd, []string{"/nonexistent/path/file.ifc"})
	if err == nil {
		t.Fatal("expected error for nonexistent file, got nil")
	}
}

func TestProgressReporterJSON(t *testing.T) {
	var buf bytes.Buffer
	p := NewProgressReporter(&buf, 1000, false, true)
	// Force an update by setting lastUpdate to the past
	p.lastUpdate = p.startTime.Add(-time.Second)
	p.Update(500, 10)
	p.Finish(20, 1)

	lines := bytes.Split(bytes.TrimSpace(buf.Bytes()), []byte("\n"))
	if len(lines) < 2 {
		t.Fatalf("expected at least 2 JSON lines, got %d: %s", len(lines), buf.String())
	}

	// Verify each line is valid JSON
	for i, line := range lines {
		if !json.Valid(line) {
			t.Errorf("line %d is not valid JSON: %s", i, string(line))
		}
	}

	// Verify finish line has done:true
	var finish map[string]any
	if err := json.Unmarshal(lines[len(lines)-1], &finish); err != nil {
		t.Fatalf("parsing finish line: %v", err)
	}
	if finish["done"] != true {
		t.Errorf("finish line missing done:true, got: %v", finish)
	}
}
