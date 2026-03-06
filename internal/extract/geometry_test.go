package extract

import (
	"database/sql"
	"encoding/json"
	"testing"

	"ifc-cli/internal/db"
)

// setupTestDB creates an in-memory DuckDB with schema and populates test entities.
func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()
	database, err := db.Open("")
	if err != nil {
		t.Fatalf("opening test db: %v", err)
	}
	t.Cleanup(func() { database.Close() })

	sqlDB := database.DB

	// Insert test entities to simulate a wall with geometry:
	//
	// #1 IFCWALL -> attrs: [GlobalId, OwnerHistory, Name, Description, ObjectType, ObjectPlacement(ref #10), Representation(ref #20)]
	// #10 IFCLOCALPLACEMENT -> attrs: [PlacementRelTo(null), RelativePlacement(ref #11)]
	// #11 IFCAXIS2PLACEMENT3D -> attrs: [Location(ref #12), Axis(null), RefDirection(null)]
	// #12 IFCCARTESIANPOINT -> attrs: [[0.0, 0.0, 0.0]]
	// #20 IFCPRODUCTDEFINITIONSHAPE -> attrs: [Name(null), Description(null), Representations([ref #30])]
	// #30 IFCSHAPEREPRESENTATION -> attrs: [ContextOfItems(ref #99), RepresentationIdentifier("Body"), RepresentationType("SweptSolid"), Items([ref #40])]
	// #40 IFCEXTRUDEDAREASOLID -> attrs: [SweptArea(ref #50), Position(ref #51), ExtrudedDirection(ref #52), Depth(3.0)]
	// #50 IFCRECTANGLEPROFILEDEF -> attrs: [ProfileType("AREA"), ProfileName(null), Position(null), XDim(5.0), YDim(0.2)]
	// #51 IFCAXIS2PLACEMENT3D -> attrs: [Location(ref #53), Axis(null), RefDirection(null)]
	// #52 IFCDIRECTION -> attrs: [[0.0, 0.0, 1.0]]
	// #53 IFCCARTESIANPOINT -> attrs: [[0.0, 0.0, 0.0]]
	// #99 IFCGEOMETRICREPRESENTATIONCONTEXT -> attrs: ["Model", "3D", 3, null, null, null]

	entities := []struct {
		id      int
		ifcType string
		attrs   string
	}{
		{1, "IFCWALL", `["abc123",null,"Wall-001",null,null,{"ref":10},{"ref":20}]`},
		{10, "IFCLOCALPLACEMENT", `[null,{"ref":11}]`},
		{11, "IFCAXIS2PLACEMENT3D", `[{"ref":12},null,null]`},
		{12, "IFCCARTESIANPOINT", `[[0.0,0.0,0.0]]`},
		{20, "IFCPRODUCTDEFINITIONSHAPE", `[null,null,[{"ref":30}]]`},
		{30, "IFCSHAPEREPRESENTATION", `[{"ref":99},"Body","SweptSolid",[{"ref":40}]]`},
		{40, "IFCEXTRUDEDAREASOLID", `[{"ref":50},{"ref":51},{"ref":52},3.0]`},
		{50, "IFCRECTANGLEPROFILEDEF", `["AREA",null,null,5.0,0.2]`},
		{51, "IFCAXIS2PLACEMENT3D", `[{"ref":53},null,null]`},
		{52, "IFCDIRECTION", `[[0.0,0.0,1.0]]`},
		{53, "IFCCARTESIANPOINT", `[[0.0,0.0,0.0]]`},
		{99, "IFCGEOMETRICREPRESENTATIONCONTEXT", `["Model","3D",3,null,null,null]`},
	}

	for _, e := range entities {
		_, err := sqlDB.Exec("INSERT INTO entities (id, ifc_type, attrs) VALUES (?, ?, ?)", e.id, e.ifcType, e.attrs)
		if err != nil {
			t.Fatalf("inserting entity #%d: %v", e.id, err)
		}
	}

	return sqlDB
}

func TestExtractGeometry_ProducesRows(t *testing.T) {
	sqlDB := setupTestDB(t)

	err := ExtractGeometry(sqlDB)
	if err != nil {
		t.Fatalf("ExtractGeometry: %v", err)
	}

	var count int
	err = sqlDB.QueryRow("SELECT COUNT(*) FROM geometry").Scan(&count)
	if err != nil {
		t.Fatalf("counting geometry rows: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 geometry row, got %d", count)
	}
}

func TestExtractGeometry_RepresentationType(t *testing.T) {
	sqlDB := setupTestDB(t)

	err := ExtractGeometry(sqlDB)
	if err != nil {
		t.Fatalf("ExtractGeometry: %v", err)
	}

	var repType string
	err = sqlDB.QueryRow("SELECT representation_type FROM geometry WHERE element_id = 1").Scan(&repType)
	if err != nil {
		t.Fatalf("querying representation_type: %v", err)
	}
	if repType != "SweptSolid" {
		t.Errorf("expected representation_type 'SweptSolid', got %q", repType)
	}
}

func TestExtractGeometry_RepresentationJSON(t *testing.T) {
	sqlDB := setupTestDB(t)

	err := ExtractGeometry(sqlDB)
	if err != nil {
		t.Fatalf("ExtractGeometry: %v", err)
	}

	var repJSON string
	err = sqlDB.QueryRow("SELECT CAST(representation_json AS VARCHAR) FROM geometry WHERE element_id = 1").Scan(&repJSON)
	if err != nil {
		t.Fatalf("querying representation_json: %v", err)
	}

	// Verify it's valid JSON
	var tree map[string]interface{}
	if err := json.Unmarshal([]byte(repJSON), &tree); err != nil {
		t.Fatalf("representation_json is not valid JSON: %v", err)
	}

	// Check structure: should have id, type, attrs
	if _, ok := tree["id"]; !ok {
		t.Error("representation_json missing 'id' field")
	}
	if tp, ok := tree["type"]; !ok {
		t.Error("representation_json missing 'type' field")
	} else if tp != "IFCSHAPEREPRESENTATION" {
		t.Errorf("expected type 'IFCSHAPEREPRESENTATION', got %v", tp)
	}
	if _, ok := tree["attrs"]; !ok {
		t.Error("representation_json missing 'attrs' field")
	}
}

func TestExtractGeometry_PlacementJSON(t *testing.T) {
	sqlDB := setupTestDB(t)

	err := ExtractGeometry(sqlDB)
	if err != nil {
		t.Fatalf("ExtractGeometry: %v", err)
	}

	var placementJSON sql.NullString
	err = sqlDB.QueryRow("SELECT CAST(placement_json AS VARCHAR) FROM geometry WHERE element_id = 1").Scan(&placementJSON)
	if err != nil {
		t.Fatalf("querying placement_json: %v", err)
	}

	if !placementJSON.Valid {
		t.Fatal("placement_json is NULL, expected a value")
	}

	// Verify it's a valid JSON array
	var chain []map[string]interface{}
	if err := json.Unmarshal([]byte(placementJSON.String), &chain); err != nil {
		t.Fatalf("placement_json is not valid JSON array: %v", err)
	}

	if len(chain) == 0 {
		t.Error("placement chain is empty")
	}

	// First element should be the IFCLOCALPLACEMENT
	if chain[0]["type"] != "IFCLOCALPLACEMENT" {
		t.Errorf("expected first placement type 'IFCLOCALPLACEMENT', got %v", chain[0]["type"])
	}

	// Should have a relative_placement with the IFCAXIS2PLACEMENT3D subtree
	if _, ok := chain[0]["relative_placement"]; !ok {
		t.Error("placement missing 'relative_placement' field")
	}
}

func TestExtractGeometry_ElementType(t *testing.T) {
	sqlDB := setupTestDB(t)

	err := ExtractGeometry(sqlDB)
	if err != nil {
		t.Fatalf("ExtractGeometry: %v", err)
	}

	var elemType string
	err = sqlDB.QueryRow("SELECT element_type FROM geometry WHERE element_id = 1").Scan(&elemType)
	if err != nil {
		t.Fatalf("querying element_type: %v", err)
	}
	if elemType != "IFCWALL" {
		t.Errorf("expected element_type 'IFCWALL', got %q", elemType)
	}
}

func TestExtractGeometry_NoGeometryElements(t *testing.T) {
	database, err := db.Open("")
	if err != nil {
		t.Fatalf("opening test db: %v", err)
	}
	t.Cleanup(func() { database.Close() })

	// Insert an entity that isn't a building element
	_, err = database.DB.Exec("INSERT INTO entities (id, ifc_type, attrs) VALUES (1, 'IFCOWNERHISTORY', '[null]')")
	if err != nil {
		t.Fatalf("inserting entity: %v", err)
	}

	err = ExtractGeometry(database.DB)
	if err != nil {
		t.Fatalf("ExtractGeometry: %v", err)
	}

	var count int
	err = database.DB.QueryRow("SELECT COUNT(*) FROM geometry").Scan(&count)
	if err != nil {
		t.Fatalf("counting geometry rows: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 geometry rows, got %d", count)
	}
}

func TestCollectSubtree_DepthLimit(t *testing.T) {
	sqlDB := setupTestDB(t)

	// Test that depth limit prevents runaway recursion
	visited := make(map[uint64]bool)
	tree, err := collectSubtree(sqlDB, 30, visited, 49)
	if err != nil {
		t.Fatalf("collectSubtree: %v", err)
	}

	// At depth 49, the root is collected but children at depth 50 hit the limit
	if tree["type"] != "IFCSHAPEREPRESENTATION" {
		t.Errorf("expected type IFCSHAPEREPRESENTATION, got %v", tree["type"])
	}
}

func TestCollectSubtree_CycleDetection(t *testing.T) {
	sqlDB := setupTestDB(t)

	// Pre-visit entity #40 to simulate a cycle
	visited := map[uint64]bool{40: true}
	tree, err := collectSubtree(sqlDB, 30, visited, 0)
	if err != nil {
		t.Fatalf("collectSubtree: %v", err)
	}

	// The tree should still be built, but #40 should show circular_ref
	attrs, ok := tree["attrs"].([]interface{})
	if !ok {
		t.Fatal("attrs not found or wrong type")
	}

	// The Items attr (index 3) is a list containing ref #40
	items, ok := attrs[3].([]interface{})
	if !ok {
		t.Fatal("items attr not a list")
	}
	if len(items) > 0 {
		item, ok := items[0].(map[string]interface{})
		if ok {
			if _, hasCycleFlag := item["circular_ref"]; !hasCycleFlag {
				t.Error("expected circular_ref flag on already-visited entity")
			}
		}
	}
}
