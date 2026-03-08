package db

import (
	"testing"

	"ifc-cli/internal/step"
)

func makeEntity(id uint64, ifcType string) *step.Entity {
	return &step.Entity{
		ID:   id,
		Type: ifcType,
		Attrs: []step.StepValue{
			{Kind: step.KindString, Str: "test"},
		},
	}
}

func TestWriterSingleEntity(t *testing.T) {
	d, err := Open("")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer d.Close()

	w, err := NewWriter(d, 100)
	if err != nil {
		t.Fatalf("NewWriter: %v", err)
	}

	if err := w.Write(makeEntity(1, "IFCWALL")); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if err := w.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	var id uint32
	var ifcType string
	err = d.DB.QueryRow("SELECT id, ifc_type FROM entities WHERE id = 1").Scan(&id, &ifcType)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if id != 1 || ifcType != "IFCWALL" {
		t.Errorf("got (%d, %s), want (1, IFCWALL)", id, ifcType)
	}
}

func TestWriterAutoFlush(t *testing.T) {
	d, err := Open("")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer d.Close()

	batchSize := 5
	w, err := NewWriter(d, batchSize)
	if err != nil {
		t.Fatalf("NewWriter: %v", err)
	}

	// Write exactly batchSize entities to trigger auto-flush
	for i := 1; i <= batchSize; i++ {
		if err := w.Write(makeEntity(uint64(i), "IFCWALL")); err != nil {
			t.Fatalf("Write %d: %v", i, err)
		}
	}

	// Close without explicit Flush — the auto-flush should have written them
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	var count int
	err = d.DB.QueryRow("SELECT COUNT(*) FROM entities").Scan(&count)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if count != batchSize {
		t.Errorf("got %d rows, want %d", count, batchSize)
	}
}

func TestWriterMultipleEntities(t *testing.T) {
	d, err := Open("")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer d.Close()

	w, err := NewWriter(d, 100)
	if err != nil {
		t.Fatalf("NewWriter: %v", err)
	}

	types := []string{"IFCWALL", "IFCSLAB", "IFCBEAM", "IFCCOLUMN", "IFCDOOR"}
	for i, tp := range types {
		if err := w.Write(makeEntity(uint64(i+1), tp)); err != nil {
			t.Fatalf("Write %d: %v", i+1, err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	var count int
	err = d.DB.QueryRow("SELECT COUNT(*) FROM entities").Scan(&count)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if count != len(types) {
		t.Errorf("got %d rows, want %d", count, len(types))
	}

	// Verify each entity
	rows, err := d.DB.Query("SELECT id, ifc_type FROM entities ORDER BY id")
	if err != nil {
		t.Fatalf("query rows: %v", err)
	}
	defer rows.Close()

	i := 0
	for rows.Next() {
		var id uint32
		var ifcType string
		if err := rows.Scan(&id, &ifcType); err != nil {
			t.Fatalf("scan: %v", err)
		}
		if int(id) != i+1 || ifcType != types[i] {
			t.Errorf("row %d: got (%d, %s), want (%d, %s)", i, id, ifcType, i+1, types[i])
		}
		i++
	}
}

func TestWriterCloseFlushesRemaining(t *testing.T) {
	d, err := Open("")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer d.Close()

	w, err := NewWriter(d, 100) // large batch size so nothing auto-flushes
	if err != nil {
		t.Fatalf("NewWriter: %v", err)
	}

	for i := 1; i <= 3; i++ {
		if err := w.Write(makeEntity(uint64(i), "IFCWALL")); err != nil {
			t.Fatalf("Write %d: %v", i, err)
		}
	}

	// Close should flush the remaining 3 entities
	if err := w.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	var count int
	err = d.DB.QueryRow("SELECT COUNT(*) FROM entities").Scan(&count)
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if count != 3 {
		t.Errorf("got %d rows, want 3", count)
	}
}
