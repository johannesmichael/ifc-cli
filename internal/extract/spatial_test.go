package extract

import (
	"database/sql"
	"testing"

	"ifc-cli/internal/step"
)

func TestExtractSpatialHierarchy_BasicTree(t *testing.T) {
	// Build: Project -> Site -> Building -> Storey -> Space
	project := &step.Entity{ID: 1, Type: "IFCPROJECT", Attrs: makeIFCRoot("gp", "My Project")}
	site := &step.Entity{ID: 2, Type: "IFCSITE", Attrs: makeIFCRoot("gs", "Site A")}
	building := &step.Entity{ID: 3, Type: "IFCBUILDING", Attrs: makeIFCRoot("gb", "Building A")}
	storey := &step.Entity{ID: 4, Type: "IFCBUILDINGSTOREY", Attrs: makeIFCRoot("gst", "Floor 2")}
	space := &step.Entity{ID: 5, Type: "IFCSPACE", Attrs: makeIFCRoot("gsp", "Room 201")}

	// Aggregation: Project->Site, Site->Building, Building->Storey, Storey->Space
	relPS := &step.Entity{ID: 100, Type: "IFCRELAGGREGATES", Attrs: makeIFCRoot("r1", "", ref(1), list(ref(2)))}
	relSB := &step.Entity{ID: 101, Type: "IFCRELAGGREGATES", Attrs: makeIFCRoot("r2", "", ref(2), list(ref(3)))}
	relBSt := &step.Entity{ID: 102, Type: "IFCRELAGGREGATES", Attrs: makeIFCRoot("r3", "", ref(3), list(ref(4)))}
	relStSp := &step.Entity{ID: 103, Type: "IFCRELAGGREGATES", Attrs: makeIFCRoot("r4", "", ref(4), list(ref(5)))}

	sqlDB := setupRelTestDB(t, project, site, building, storey, space, relPS, relSB, relBSt, relStSp)

	cache, err := NewEntityCache(sqlDB)
	if err != nil {
		t.Fatalf("NewEntityCache: %v", err)
	}
	if err := ExtractRelationships(sqlDB, cache); err != nil {
		t.Fatalf("ExtractRelationships: %v", err)
	}
	if err := ExtractSpatialHierarchy(sqlDB, cache); err != nil {
		t.Fatalf("ExtractSpatialHierarchy: %v", err)
	}

	// Verify spatial_structure entries
	rows, err := sqlDB.Query("SELECT element_id, element_type, element_name, parent_id, hierarchy_level, path FROM spatial_structure ORDER BY hierarchy_level, element_id")
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	defer rows.Close()

	type spatialRow struct {
		elemID   uint32
		elemType string
		elemName sql.NullString
		parentID sql.NullInt64
		level    int
		path     string
	}

	var results []spatialRow
	for rows.Next() {
		var r spatialRow
		if err := rows.Scan(&r.elemID, &r.elemType, &r.elemName, &r.parentID, &r.level, &r.path); err != nil {
			t.Fatalf("scan: %v", err)
		}
		results = append(results, r)
	}

	if len(results) != 5 {
		t.Fatalf("got %d spatial rows, want 5", len(results))
	}

	// Check project (root, level 0)
	if results[0].elemID != 1 || results[0].level != 0 {
		t.Errorf("project: got id=%d level=%d, want id=1 level=0", results[0].elemID, results[0].level)
	}
	if results[0].path != "My Project" {
		t.Errorf("project path: got %q, want %q", results[0].path, "My Project")
	}
	if results[0].parentID.Valid {
		t.Errorf("project should have no parent")
	}

	// Check storey (level 3)
	if results[3].elemID != 4 || results[3].level != 3 {
		t.Errorf("storey: got id=%d level=%d, want id=4 level=3", results[3].elemID, results[3].level)
	}
	expectedPath := "My Project/Site A/Building A/Floor 2"
	if results[3].path != expectedPath {
		t.Errorf("storey path: got %q, want %q", results[3].path, expectedPath)
	}

	// Check space (level 4)
	if results[4].elemID != 5 || results[4].level != 4 {
		t.Errorf("space: got id=%d level=%d, want id=5 level=4", results[4].elemID, results[4].level)
	}
	expectedPath = "My Project/Site A/Building A/Floor 2/Room 201"
	if results[4].path != expectedPath {
		t.Errorf("space path: got %q, want %q", results[4].path, expectedPath)
	}
}

func TestExtractSpatialHierarchy_Containment(t *testing.T) {
	// Storey contains a Wall
	storey := &step.Entity{ID: 4, Type: "IFCBUILDINGSTOREY", Attrs: makeIFCRoot("gst", "Floor 1")}
	wall := &step.Entity{ID: 30, Type: "IFCWALL", Attrs: makeIFCRoot("gw", "Ext Wall")}
	project := &step.Entity{ID: 1, Type: "IFCPROJECT", Attrs: makeIFCRoot("gp", "Proj")}

	// Project -> Storey (simplified, skip site/building for test)
	relAgg := &step.Entity{ID: 100, Type: "IFCRELAGGREGATES", Attrs: makeIFCRoot("r1", "", ref(1), list(ref(4)))}

	// Containment: Wall in Storey
	relContain := &step.Entity{
		ID:   101,
		Type: "IFCRELCONTAINEDINSPATIALSTRUCTURE",
		Attrs: makeIFCRoot("rc", "",
			list(ref(30)), // attr[4]: elements
			ref(4),        // attr[5]: spatial container
		),
	}

	sqlDB := setupRelTestDB(t, project, storey, wall, relAgg, relContain)

	cache, err := NewEntityCache(sqlDB)
	if err != nil {
		t.Fatalf("NewEntityCache: %v", err)
	}
	if err := ExtractRelationships(sqlDB, cache); err != nil {
		t.Fatalf("ExtractRelationships: %v", err)
	}
	if err := ExtractSpatialHierarchy(sqlDB, cache); err != nil {
		t.Fatalf("ExtractSpatialHierarchy: %v", err)
	}

	// Verify the wall appears in spatial_structure with storey as parent
	var elemType, path string
	var parentID sql.NullInt64
	var level int
	err = sqlDB.QueryRow(
		"SELECT element_type, parent_id, hierarchy_level, path FROM spatial_structure WHERE element_id = 30",
	).Scan(&elemType, &parentID, &level, &path)
	if err != nil {
		t.Fatalf("query wall: %v", err)
	}

	if elemType != "IFCWALL" {
		t.Errorf("wall type: got %s, want IFCWALL", elemType)
	}
	if !parentID.Valid || parentID.Int64 != 4 {
		t.Errorf("wall parent: got %v, want 4", parentID)
	}
	// Level should be storey.level + 1 = 2
	if level != 2 {
		t.Errorf("wall level: got %d, want 2", level)
	}
	expectedPath := "Proj/Floor 1/Ext Wall"
	if path != expectedPath {
		t.Errorf("wall path: got %q, want %q", path, expectedPath)
	}
}

func TestExtractSpatialHierarchy_BidirectionalTraversal(t *testing.T) {
	// Test that we can traverse element->container and container->elements via spatial_structure
	building := &step.Entity{ID: 3, Type: "IFCBUILDING", Attrs: makeIFCRoot("gb", "Bldg")}
	storey1 := &step.Entity{ID: 4, Type: "IFCBUILDINGSTOREY", Attrs: makeIFCRoot("gs1", "F1")}
	storey2 := &step.Entity{ID: 5, Type: "IFCBUILDINGSTOREY", Attrs: makeIFCRoot("gs2", "F2")}
	project := &step.Entity{ID: 1, Type: "IFCPROJECT", Attrs: makeIFCRoot("gp", "P")}

	relPB := &step.Entity{ID: 100, Type: "IFCRELAGGREGATES", Attrs: makeIFCRoot("r1", "", ref(1), list(ref(3)))}
	relBS := &step.Entity{ID: 101, Type: "IFCRELAGGREGATES", Attrs: makeIFCRoot("r2", "", ref(3), list(ref(4), ref(5)))}

	wall := &step.Entity{ID: 30, Type: "IFCWALL", Attrs: makeIFCRoot("gw", "W1")}
	relContain := &step.Entity{
		ID:   102,
		Type: "IFCRELCONTAINEDINSPATIALSTRUCTURE",
		Attrs: makeIFCRoot("rc", "",
			list(ref(30)),
			ref(4),
		),
	}

	sqlDB := setupRelTestDB(t, project, building, storey1, storey2, wall, relPB, relBS, relContain)

	cache, err := NewEntityCache(sqlDB)
	if err != nil {
		t.Fatalf("NewEntityCache: %v", err)
	}
	if err := ExtractRelationships(sqlDB, cache); err != nil {
		t.Fatalf("ExtractRelationships: %v", err)
	}
	if err := ExtractSpatialHierarchy(sqlDB, cache); err != nil {
		t.Fatalf("ExtractSpatialHierarchy: %v", err)
	}

	// Element -> container: find wall's container
	var parentID sql.NullInt64
	err = sqlDB.QueryRow("SELECT parent_id FROM spatial_structure WHERE element_id = 30").Scan(&parentID)
	if err != nil {
		t.Fatalf("element->container query: %v", err)
	}
	if !parentID.Valid || parentID.Int64 != 4 {
		t.Errorf("wall's container: got %v, want 4", parentID)
	}

	// Container -> elements: find everything with parent_id = building(3)
	rows, err := sqlDB.Query("SELECT element_id FROM spatial_structure WHERE parent_id = 3 ORDER BY element_id")
	if err != nil {
		t.Fatalf("container->elements query: %v", err)
	}
	defer rows.Close()

	var children []uint32
	for rows.Next() {
		var id uint32
		if err := rows.Scan(&id); err != nil {
			t.Fatalf("scan: %v", err)
		}
		children = append(children, id)
	}

	if len(children) != 2 {
		t.Fatalf("building children: got %d, want 2", len(children))
	}
	if children[0] != 4 || children[1] != 5 {
		t.Errorf("building children: got %v, want [4, 5]", children)
	}
}

func TestExtractSpatialHierarchy_EmptyRelationships(t *testing.T) {
	// No relationships → no spatial structure entries
	sqlDB := setupRelTestDB(t,
		&step.Entity{ID: 1, Type: "IFCWALL", Attrs: makeIFCRoot("g", "W")},
	)

	cache, err := NewEntityCache(sqlDB)
	if err != nil {
		t.Fatalf("NewEntityCache: %v", err)
	}
	if err := ExtractSpatialHierarchy(sqlDB, cache); err != nil {
		t.Fatalf("ExtractSpatialHierarchy: %v", err)
	}

	var count int
	if err := sqlDB.QueryRow("SELECT COUNT(*) FROM spatial_structure").Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 0 {
		t.Errorf("got %d spatial rows, want 0", count)
	}
}
