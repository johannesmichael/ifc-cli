package extract

import (
	"database/sql"
	"encoding/json"
	"testing"

	"ifc-cli/internal/db"
	"ifc-cli/internal/step"
)

// setupRelTestDB creates an in-memory DuckDB with schema and inserts the given entities.
// It returns the sql.DB and an EntityCache populated with in-memory step attrs.
func setupRelTestDB(t *testing.T, entities ...*step.Entity) (*sql.DB, *EntityCache) {
	t.Helper()

	d, err := db.Open("")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { d.Close() })

	w, err := db.NewWriter(d, 100)
	if err != nil {
		t.Fatalf("new writer: %v", err)
	}
	for _, e := range entities {
		if err := w.Write(e); err != nil {
			t.Fatalf("write entity %d: %v", e.ID, err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}

	// Build cache with in-memory step attrs
	cache := NewEntityCacheEmpty()
	for _, e := range entities {
		globalID := ""
		if len(e.Attrs) > 0 && e.Attrs[0].Kind == step.KindString {
			globalID = e.Attrs[0].Str
		}
		cache.Put(e.ID, e.Type, globalID, e.Attrs)
	}

	return d.DB, cache
}

// ref creates a StepValue ref.
func ref(id uint64) step.StepValue {
	return step.StepValue{Kind: step.KindRef, Ref: id}
}

// str creates a StepValue string.
func str(s string) step.StepValue {
	return step.StepValue{Kind: step.KindString, Str: s}
}

// null creates a null StepValue.
func null() step.StepValue {
	return step.StepValue{Kind: step.KindNull}
}

// list creates a StepValue list.
func list(vals ...step.StepValue) step.StepValue {
	return step.StepValue{Kind: step.KindList, List: vals}
}

// enum creates a StepValue enum.
func enum(s string) step.StepValue {
	return step.StepValue{Kind: step.KindEnum, Str: s}
}

// makeIFCRoot creates IFCROOT-like attrs: GlobalId, OwnerHistory, Name, Description + extra attrs.
func makeIFCRoot(globalID, name string, extra ...step.StepValue) []step.StepValue {
	attrs := []step.StepValue{
		str(globalID), // 0: GlobalId
		null(),        // 1: OwnerHistory
		str(name),     // 2: Name
		null(),        // 3: Description
	}
	attrs = append(attrs, extra...)
	return attrs
}

func TestExtractRelationships_Aggregation(t *testing.T) {
	// IFCRELAGGREGATES: attr[4]=RelatingObject (single ref), attr[5]=RelatedObjects (list of refs)
	building := &step.Entity{ID: 10, Type: "IFCBUILDING", Attrs: makeIFCRoot("guid-b", "Building A")}
	storey1 := &step.Entity{ID: 20, Type: "IFCBUILDINGSTOREY", Attrs: makeIFCRoot("guid-s1", "Floor 1")}
	storey2 := &step.Entity{ID: 21, Type: "IFCBUILDINGSTOREY", Attrs: makeIFCRoot("guid-s2", "Floor 2")}

	relAgg := &step.Entity{
		ID:   100,
		Type: "IFCRELAGGREGATES",
		Attrs: makeIFCRoot("guid-ra", "Aggregation",
			ref(10),                  // attr[4]: RelatingObject = Building
			list(ref(20), ref(21)),   // attr[5]: RelatedObjects = [Storey1, Storey2]
		),
	}

	sqlDB, cache := setupRelTestDB(t, building, storey1, storey2, relAgg)

	if err := ExtractRelationships(sqlDB, cache, false, nil); err != nil {
		t.Fatalf("ExtractRelationships: %v", err)
	}

	// Should produce 2 rows: (100, IFCRELAGGREGATES, 10, 20) and (100, IFCRELAGGREGATES, 10, 21)
	rows, err := sqlDB.Query("SELECT rel_id, rel_type, source_id, target_id, context FROM relationships ORDER BY target_id")
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	defer rows.Close()

	type relRow struct {
		relID, sourceID, targetID uint32
		relType                   string
		context                   sql.NullString
	}
	var results []relRow
	for rows.Next() {
		var r relRow
		if err := rows.Scan(&r.relID, &r.relType, &r.sourceID, &r.targetID, &r.context); err != nil {
			t.Fatalf("scan: %v", err)
		}
		results = append(results, r)
	}

	if len(results) != 2 {
		t.Fatalf("got %d rows, want 2", len(results))
	}

	// Verify first row: Building -> Storey1
	if results[0].sourceID != 10 || results[0].targetID != 20 {
		t.Errorf("row 0: got source=%d target=%d, want source=10 target=20", results[0].sourceID, results[0].targetID)
	}
	if results[0].relType != "IFCRELAGGREGATES" {
		t.Errorf("row 0: got type=%s, want IFCRELAGGREGATES", results[0].relType)
	}

	// Verify second row: Building -> Storey2
	if results[1].sourceID != 10 || results[1].targetID != 21 {
		t.Errorf("row 1: got source=%d target=%d, want source=10 target=21", results[1].sourceID, results[1].targetID)
	}
}

func TestExtractRelationships_Containment(t *testing.T) {
	// IFCRELCONTAINEDINSPATIALSTRUCTURE: attr[4]=RelatedElements (list), attr[5]=RelatingStructure (single ref)
	storey := &step.Entity{ID: 20, Type: "IFCBUILDINGSTOREY", Attrs: makeIFCRoot("guid-s", "Floor 1")}
	wall := &step.Entity{ID: 30, Type: "IFCWALL", Attrs: makeIFCRoot("guid-w", "Wall 1")}
	door := &step.Entity{ID: 31, Type: "IFCDOOR", Attrs: makeIFCRoot("guid-d", "Door 1")}

	relContain := &step.Entity{
		ID:   101,
		Type: "IFCRELCONTAINEDINSPATIALSTRUCTURE",
		Attrs: makeIFCRoot("guid-rc", "Containment",
			list(ref(30), ref(31)), // attr[4]: RelatedElements = [Wall, Door]
			ref(20),                // attr[5]: RelatingStructure = Storey
		),
	}

	sqlDB, cache := setupRelTestDB(t, storey, wall, door, relContain)

	if err := ExtractRelationships(sqlDB, cache, false, nil); err != nil {
		t.Fatalf("ExtractRelationships: %v", err)
	}

	var count int
	if err := sqlDB.QueryRow("SELECT COUNT(*) FROM relationships").Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 2 {
		t.Fatalf("got %d rows, want 2", count)
	}

	// Verify: each element paired with the spatial structure
	rows, err := sqlDB.Query("SELECT source_id, target_id FROM relationships ORDER BY source_id")
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	defer rows.Close()

	expected := [][2]uint32{{30, 20}, {31, 20}}
	i := 0
	for rows.Next() {
		var src, tgt uint32
		if err := rows.Scan(&src, &tgt); err != nil {
			t.Fatalf("scan: %v", err)
		}
		if src != expected[i][0] || tgt != expected[i][1] {
			t.Errorf("row %d: got (%d, %d), want (%d, %d)", i, src, tgt, expected[i][0], expected[i][1])
		}
		i++
	}
}

func TestExtractRelationships_UnknownRelType(t *testing.T) {
	// Unknown IFCREL type should be skipped
	relUnknown := &step.Entity{
		ID:   200,
		Type: "IFCRELUNKNOWN",
		Attrs: makeIFCRoot("guid-u", "Unknown",
			ref(1),
			ref(2),
		),
	}

	sqlDB, cache := setupRelTestDB(t, relUnknown)

	if err := ExtractRelationships(sqlDB, cache, false, nil); err != nil {
		t.Fatalf("ExtractRelationships: %v", err)
	}

	var count int
	if err := sqlDB.QueryRow("SELECT COUNT(*) FROM relationships").Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 0 {
		t.Errorf("got %d rows, want 0 for unknown rel type", count)
	}
}

func TestExtractRelationships_Context(t *testing.T) {
	// Verify context is set for known relationship types
	relAgg := &step.Entity{
		ID:   100,
		Type: "IFCRELAGGREGATES",
		Attrs: makeIFCRoot("guid-ra", "Agg",
			ref(1),
			list(ref(2)),
		),
	}

	sqlDB, cache := setupRelTestDB(t,
		&step.Entity{ID: 1, Type: "IFCBUILDING", Attrs: makeIFCRoot("g1", "B")},
		&step.Entity{ID: 2, Type: "IFCBUILDINGSTOREY", Attrs: makeIFCRoot("g2", "S")},
		relAgg,
	)

	if err := ExtractRelationships(sqlDB, cache, false, nil); err != nil {
		t.Fatalf("ExtractRelationships: %v", err)
	}

	var context sql.NullString
	if err := sqlDB.QueryRow("SELECT context FROM relationships LIMIT 1").Scan(&context); err != nil {
		t.Fatalf("query: %v", err)
	}
	if !context.Valid || context.String != "aggregation" {
		t.Errorf("got context=%v, want 'aggregation'", context)
	}
}

func TestExtractRefs(t *testing.T) {
	// Test extractRefs with various JSON structures
	tests := []struct {
		name string
		val  interface{}
		want int
	}{
		{"single ref", map[string]interface{}{"ref": float64(42)}, 1},
		{"list of refs", []interface{}{
			map[string]interface{}{"ref": float64(1)},
			map[string]interface{}{"ref": float64(2)},
		}, 2},
		{"null", nil, 0},
		{"string", "hello", 0},
		{"empty list", []interface{}{}, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			refs := extractRefs(tt.val)
			if len(refs) != tt.want {
				t.Errorf("extractRefs: got %d refs, want %d", len(refs), tt.want)
			}
		})
	}
}

// Ensure JSON round-trip works for our test helpers.
func TestJSONRoundTrip(t *testing.T) {
	e := &step.Entity{
		ID:   1,
		Type: "IFCRELAGGREGATES",
		Attrs: makeIFCRoot("guid", "Name",
			ref(10),
			list(ref(20), ref(30)),
		),
	}
	data, err := step.MarshalAttrs(e.Attrs)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var attrs []interface{}
	if err := json.Unmarshal(data, &attrs); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// attr[4] should be a ref to 10
	sourceRefs := extractRefs(attrs[4])
	if len(sourceRefs) != 1 || sourceRefs[0] != 10 {
		t.Errorf("source refs: got %v, want [10]", sourceRefs)
	}

	// attr[5] should be a list of refs [20, 30]
	targetRefs := extractRefs(attrs[5])
	if len(targetRefs) != 2 || targetRefs[0] != 20 || targetRefs[1] != 30 {
		t.Errorf("target refs: got %v, want [20, 30]", targetRefs)
	}
}
