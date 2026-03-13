package extract

import (
	"os"
	"path/filepath"
	"testing"

	"ifc-cli/internal/db"
	"ifc-cli/internal/step"
)

func importTestFile(t *testing.T, filename string) *db.Database {
	t.Helper()
	testFile := filepath.Join("..", "step", "testdata", filename)
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

	parser := step.NewParser(data)
	writer, err := db.NewWriter(database, 100)
	if err != nil {
		database.Close()
		t.Fatalf("creating writer: %v", err)
	}

	for {
		entity, err := parser.Next()
		if err != nil {
			break
		}
		if err := writer.Write(entity); err != nil {
			database.Close()
			t.Fatalf("writing entity: %v", err)
		}
	}
	if err := writer.Close(); err != nil {
		database.Close()
		t.Fatalf("closing writer: %v", err)
	}

	return database
}

func TestExtractProperties(t *testing.T) {
	database := importTestFile(t, "wall_with_properties.ifc")
	defer database.Close()

	cache, err := NewEntityCache(database.DB)
	if err != nil {
		t.Fatalf("NewEntityCache: %v", err)
	}
	err = ExtractProperties(database.DB, cache, false, nil)
	if err != nil {
		t.Fatalf("ExtractProperties: %v", err)
	}

	// Verify properties were extracted
	var count int
	err = database.DB.QueryRow("SELECT COUNT(*) FROM properties").Scan(&count)
	if err != nil {
		t.Fatalf("querying property count: %v", err)
	}
	if count == 0 {
		t.Fatal("no properties extracted")
	}

	// The wall_with_properties.ifc has 3 properties: FireRating, IsExternal, ThermalTransmittance
	if count != 3 {
		t.Errorf("got %d properties, want 3", count)
	}
}

func TestExtractPropertiesValues(t *testing.T) {
	database := importTestFile(t, "wall_with_properties.ifc")
	defer database.Close()

	cache, err := NewEntityCache(database.DB)
	if err != nil {
		t.Fatalf("NewEntityCache: %v", err)
	}
	err = ExtractProperties(database.DB, cache, false, nil)
	if err != nil {
		t.Fatalf("ExtractProperties: %v", err)
	}

	// Check FireRating property
	var propValue, valueType, psetName string
	var elementID uint64
	err = database.DB.QueryRow(
		"SELECT element_id, pset_name, prop_value, value_type FROM properties WHERE prop_name = 'FireRating'",
	).Scan(&elementID, &psetName, &propValue, &valueType)
	if err != nil {
		t.Fatalf("querying FireRating: %v", err)
	}

	if elementID != 20 {
		t.Errorf("FireRating element_id = %d, want 20", elementID)
	}
	if psetName != "Pset_WallCommon" {
		t.Errorf("FireRating pset_name = %q, want %q", psetName, "Pset_WallCommon")
	}
	if propValue != "2 hours" {
		t.Errorf("FireRating prop_value = %q, want %q", propValue, "2 hours")
	}
	if valueType != "IFCLABEL" {
		t.Errorf("FireRating value_type = %q, want %q", valueType, "IFCLABEL")
	}
}

func TestExtractPropertiesIsExternal(t *testing.T) {
	database := importTestFile(t, "wall_with_properties.ifc")
	defer database.Close()

	cache, err := NewEntityCache(database.DB)
	if err != nil {
		t.Fatalf("NewEntityCache: %v", err)
	}
	err = ExtractProperties(database.DB, cache, false, nil)
	if err != nil {
		t.Fatalf("ExtractProperties: %v", err)
	}

	var propValue, valueType string
	err = database.DB.QueryRow(
		"SELECT prop_value, value_type FROM properties WHERE prop_name = 'IsExternal'",
	).Scan(&propValue, &valueType)
	if err != nil {
		t.Fatalf("querying IsExternal: %v", err)
	}

	if propValue != "true" {
		t.Errorf("IsExternal prop_value = %q, want %q", propValue, "true")
	}
	if valueType != "IFCBOOLEAN" {
		t.Errorf("IsExternal value_type = %q, want %q", valueType, "IFCBOOLEAN")
	}
}

func TestExtractPropertiesThermalTransmittance(t *testing.T) {
	database := importTestFile(t, "wall_with_properties.ifc")
	defer database.Close()

	cache, err := NewEntityCache(database.DB)
	if err != nil {
		t.Fatalf("NewEntityCache: %v", err)
	}
	err = ExtractProperties(database.DB, cache, false, nil)
	if err != nil {
		t.Fatalf("ExtractProperties: %v", err)
	}

	var propValue, valueType string
	err = database.DB.QueryRow(
		"SELECT prop_value, value_type FROM properties WHERE prop_name = 'ThermalTransmittance'",
	).Scan(&propValue, &valueType)
	if err != nil {
		t.Fatalf("querying ThermalTransmittance: %v", err)
	}

	if propValue != "0.24" {
		t.Errorf("ThermalTransmittance prop_value = %q, want %q", propValue, "0.24")
	}
	if valueType != "IFCTHERMALTRANSMITTANCEMEASURE" {
		t.Errorf("ThermalTransmittance value_type = %q, want %q", valueType, "IFCTHERMALTRANSMITTANCEMEASURE")
	}
}

func TestExtractPropertiesElementType(t *testing.T) {
	database := importTestFile(t, "wall_with_properties.ifc")
	defer database.Close()

	cache, err := NewEntityCache(database.DB)
	if err != nil {
		t.Fatalf("NewEntityCache: %v", err)
	}
	err = ExtractProperties(database.DB, cache, false, nil)
	if err != nil {
		t.Fatalf("ExtractProperties: %v", err)
	}

	var elementType string
	err = database.DB.QueryRow(
		"SELECT DISTINCT element_type FROM properties WHERE element_id = 20",
	).Scan(&elementType)
	if err != nil {
		t.Fatalf("querying element_type: %v", err)
	}

	if elementType != "IFCWALL" {
		t.Errorf("element_type = %q, want %q", elementType, "IFCWALL")
	}
}

func TestExtractPropertiesEmptyDB(t *testing.T) {
	database, err := db.Open("")
	if err != nil {
		t.Fatalf("opening database: %v", err)
	}
	defer database.Close()

	cache, err := NewEntityCache(database.DB)
	if err != nil {
		t.Fatalf("NewEntityCache: %v", err)
	}
	err = ExtractProperties(database.DB, cache, false, nil)
	if err != nil {
		t.Fatalf("ExtractProperties on empty DB should not error: %v", err)
	}

	var count int
	err = database.DB.QueryRow("SELECT COUNT(*) FROM properties").Scan(&count)
	if err != nil {
		t.Fatalf("querying property count: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 properties on empty DB, got %d", count)
	}
}

func TestStepRefHelper(t *testing.T) {
	tests := []struct {
		input step.StepValue
		want  uint64
		ok    bool
	}{
		{step.StepValue{Kind: step.KindRef, Ref: 123}, 123, true},
		{step.StepValue{Kind: step.KindRef, Ref: 0}, 0, true},
		{step.StepValue{Kind: step.KindString, Str: "hello"}, 0, false},
		{step.StepValue{Kind: step.KindNull}, 0, false},
		{step.StepValue{Kind: step.KindInteger, Int: 42}, 0, false},
	}

	for _, tt := range tests {
		got, ok := StepRef(tt.input)
		if ok != tt.ok || got != tt.want {
			t.Errorf("StepRef(%v) = (%d, %v), want (%d, %v)", tt.input, got, ok, tt.want, tt.ok)
		}
	}
}

func TestStepRefListHelper(t *testing.T) {
	input := step.StepValue{Kind: step.KindList, List: []step.StepValue{
		{Kind: step.KindRef, Ref: 10},
		{Kind: step.KindRef, Ref: 20},
		{Kind: step.KindRef, Ref: 30},
	}}
	refs := StepRefList(input)
	if len(refs) != 3 {
		t.Fatalf("StepRefList got %d refs, want 3", len(refs))
	}
	if refs[0] != 10 || refs[1] != 20 || refs[2] != 30 {
		t.Errorf("StepRefList = %v, want [10, 20, 30]", refs)
	}
}

func TestStepFormatValueTyped(t *testing.T) {
	inner := step.StepValue{Kind: step.KindString, Str: "2 hours"}
	input := step.StepValue{Kind: step.KindTyped, Str: "IFCLABEL", Inner: &inner}
	typeName := StepFormatValueType(input)
	val := StepFormatValue(input)
	if typeName != "IFCLABEL" {
		t.Errorf("type = %q, want IFCLABEL", typeName)
	}
	if val != "2 hours" {
		t.Errorf("value = %q, want '2 hours'", val)
	}
}

func TestStepFormatValueEnum(t *testing.T) {
	input := step.StepValue{Kind: step.KindEnum, Str: ".T."}
	val := StepFormatValue(input)
	if val != "true" {
		t.Errorf("StepFormatValue enum .T. = %q, want %q", val, "true")
	}
}

func TestMergeProperties(t *testing.T) {
	instance := []Property{
		{ElementID: 1, PSetName: "Pset_A", PropName: "P1", PropValue: "inst", Source: "instance"},
	}
	typeLevel := []Property{
		{ElementID: 1, PSetName: "Pset_A", PropName: "P1", PropValue: "type", Source: "type"},
		{ElementID: 1, PSetName: "Pset_A", PropName: "P2", PropValue: "type", Source: "type"},
	}

	merged := mergeProperties(instance, typeLevel)
	if len(merged) != 2 {
		t.Fatalf("merged len = %d, want 2", len(merged))
	}

	// P1 should be instance value
	for _, p := range merged {
		if p.PropName == "P1" && p.PropValue != "inst" {
			t.Errorf("P1 should be instance value 'inst', got %q", p.PropValue)
		}
		if p.PropName == "P2" && p.PropValue != "type" {
			t.Errorf("P2 should be type value 'type', got %q", p.PropValue)
		}
	}
}

func TestExtractPropertiesElementsOnly(t *testing.T) {
	database := importTestFile(t, "wall_with_properties.ifc")
	defer database.Close()

	cache, err := NewEntityCache(database.DB)
	if err != nil {
		t.Fatalf("NewEntityCache: %v", err)
	}

	// elementsOnly=true should still extract wall properties (wall has GlobalID)
	err = ExtractProperties(database.DB, cache, true, nil)
	if err != nil {
		t.Fatalf("ExtractProperties elementsOnly: %v", err)
	}

	var count int
	err = database.DB.QueryRow("SELECT COUNT(*) FROM properties").Scan(&count)
	if err != nil {
		t.Fatalf("querying property count: %v", err)
	}

	// Wall has a GlobalID, so its 3 properties should still be extracted
	if count != 3 {
		t.Errorf("got %d properties with elementsOnly=true, want 3", count)
	}
}
