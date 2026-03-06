package step

import (
	"io"
	"testing"
)

func TestParseSimpleEntity(t *testing.T) {
	src := []byte(`#1 = IFCWALL('name', 42, 3.14, .ELEMENT., #2, $, *);`)
	p := NewParser(src)

	ent, err := p.Next()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ent.ID != 1 {
		t.Errorf("expected ID 1, got %d", ent.ID)
	}
	if ent.Type != "IFCWALL" {
		t.Errorf("expected type IFCWALL, got %s", ent.Type)
	}
	if len(ent.Attrs) != 7 {
		t.Fatalf("expected 7 attrs, got %d", len(ent.Attrs))
	}

	// 'name'
	if ent.Attrs[0].Kind != KindString || ent.Attrs[0].Str != "name" {
		t.Errorf("attr[0]: expected String 'name', got %v %q", ent.Attrs[0].Kind, ent.Attrs[0].Str)
	}
	// 42
	if ent.Attrs[1].Kind != KindInteger || ent.Attrs[1].Int != 42 {
		t.Errorf("attr[1]: expected Integer 42, got %v %d", ent.Attrs[1].Kind, ent.Attrs[1].Int)
	}
	// 3.14
	if ent.Attrs[2].Kind != KindFloat || ent.Attrs[2].Float != 3.14 {
		t.Errorf("attr[2]: expected Float 3.14, got %v %f", ent.Attrs[2].Kind, ent.Attrs[2].Float)
	}
	// .ELEMENT.
	if ent.Attrs[3].Kind != KindEnum || ent.Attrs[3].Str != ".ELEMENT." {
		t.Errorf("attr[3]: expected Enum .ELEMENT., got %v %q", ent.Attrs[3].Kind, ent.Attrs[3].Str)
	}
	// #2
	if ent.Attrs[4].Kind != KindRef || ent.Attrs[4].Ref != 2 {
		t.Errorf("attr[4]: expected Ref 2, got %v %d", ent.Attrs[4].Kind, ent.Attrs[4].Ref)
	}
	// $
	if ent.Attrs[5].Kind != KindNull {
		t.Errorf("attr[5]: expected Null, got %v", ent.Attrs[5].Kind)
	}
	// *
	if ent.Attrs[6].Kind != KindDerived {
		t.Errorf("attr[6]: expected Derived, got %v", ent.Attrs[6].Kind)
	}

	// EOF
	ent, err = p.Next()
	if err != io.EOF {
		t.Errorf("expected io.EOF, got err=%v ent=%v", err, ent)
	}
}

func TestParseEmptyList(t *testing.T) {
	src := []byte(`#1 = IFCWALL(());`)
	p := NewParser(src)

	ent, err := p.Next()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ent.Attrs) != 1 {
		t.Fatalf("expected 1 attr, got %d", len(ent.Attrs))
	}
	if ent.Attrs[0].Kind != KindList {
		t.Errorf("expected List, got %v", ent.Attrs[0].Kind)
	}
	if len(ent.Attrs[0].List) != 0 {
		t.Errorf("expected empty list, got %d items", len(ent.Attrs[0].List))
	}
}

func TestParseNestedList(t *testing.T) {
	src := []byte(`#1 = IFCWALL((1,2),(3,4));`)
	p := NewParser(src)

	ent, err := p.Next()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ent.Attrs) != 2 {
		t.Fatalf("expected 2 attrs, got %d", len(ent.Attrs))
	}

	for i, attr := range ent.Attrs {
		if attr.Kind != KindList {
			t.Errorf("attr[%d]: expected List, got %v", i, attr.Kind)
			continue
		}
		if len(attr.List) != 2 {
			t.Errorf("attr[%d]: expected 2 items, got %d", i, len(attr.List))
		}
	}

	// Check values: (1,2)
	if ent.Attrs[0].List[0].Int != 1 || ent.Attrs[0].List[1].Int != 2 {
		t.Errorf("first list: expected (1,2), got (%d,%d)", ent.Attrs[0].List[0].Int, ent.Attrs[0].List[1].Int)
	}
	// Check values: (3,4)
	if ent.Attrs[1].List[0].Int != 3 || ent.Attrs[1].List[1].Int != 4 {
		t.Errorf("second list: expected (3,4), got (%d,%d)", ent.Attrs[1].List[0].Int, ent.Attrs[1].List[1].Int)
	}
}

func TestParseTypedValue(t *testing.T) {
	src := []byte(`#1 = IFCWALL(IFCLENGTHMEASURE(2.5));`)
	p := NewParser(src)

	ent, err := p.Next()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ent.Attrs) != 1 {
		t.Fatalf("expected 1 attr, got %d", len(ent.Attrs))
	}

	attr := ent.Attrs[0]
	if attr.Kind != KindTyped {
		t.Fatalf("expected Typed, got %v", attr.Kind)
	}
	if attr.Str != "IFCLENGTHMEASURE" {
		t.Errorf("expected type name IFCLENGTHMEASURE, got %q", attr.Str)
	}
	if attr.Inner == nil {
		t.Fatal("expected inner value, got nil")
	}
	if attr.Inner.Kind != KindFloat || attr.Inner.Float != 2.5 {
		t.Errorf("expected inner Float 2.5, got %v %f", attr.Inner.Kind, attr.Inner.Float)
	}
}

func TestParseRefsInList(t *testing.T) {
	src := []byte(`#1 = IFCWALL((#2,#3,#4));`)
	p := NewParser(src)

	ent, err := p.Next()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(ent.Attrs) != 1 {
		t.Fatalf("expected 1 attr, got %d", len(ent.Attrs))
	}

	list := ent.Attrs[0]
	if list.Kind != KindList {
		t.Fatalf("expected List, got %v", list.Kind)
	}
	if len(list.List) != 3 {
		t.Fatalf("expected 3 refs, got %d", len(list.List))
	}

	expectedRefs := []uint64{2, 3, 4}
	for i, ref := range list.List {
		if ref.Kind != KindRef {
			t.Errorf("list[%d]: expected Ref, got %v", i, ref.Kind)
		}
		if ref.Ref != expectedRefs[i] {
			t.Errorf("list[%d]: expected ref #%d, got #%d", i, expectedRefs[i], ref.Ref)
		}
	}
}

func TestParseMultipleEntities(t *testing.T) {
	src := []byte(`#1 = IFCWALL('a');
#2 = IFCCOLUMN('b');
#3 = IFCSLAB('c');`)
	p := NewParser(src)

	expected := []struct {
		id   uint64
		typ  string
		str  string
	}{
		{1, "IFCWALL", "a"},
		{2, "IFCCOLUMN", "b"},
		{3, "IFCSLAB", "c"},
	}

	for _, exp := range expected {
		ent, err := p.Next()
		if err != nil {
			t.Fatalf("unexpected error parsing #%d: %v", exp.id, err)
		}
		if ent.ID != exp.id {
			t.Errorf("expected ID %d, got %d", exp.id, ent.ID)
		}
		if ent.Type != exp.typ {
			t.Errorf("expected type %s, got %s", exp.typ, ent.Type)
		}
		if len(ent.Attrs) != 1 || ent.Attrs[0].Str != exp.str {
			t.Errorf("expected attr %q, got %v", exp.str, ent.Attrs)
		}
	}

	ent, err := p.Next()
	if err != io.EOF {
		t.Errorf("expected io.EOF, got err=%v ent=%v", err, ent)
	}
}

func TestParseFullSTEPFile(t *testing.T) {
	src := []byte(`ISO-10303-21;
HEADER;
FILE_DESCRIPTION(('test'),'2;1');
ENDSEC;
DATA;
#1 = IFCPROJECT('guid',$,$,$,$,$,$,$,$);
#2 = IFCWALL('guid',$,$,$,$,$,$,$);
END-ISO-10303-21;
`)
	p := NewParser(src)

	// First entity
	ent, err := p.Next()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ent.ID != 1 || ent.Type != "IFCPROJECT" {
		t.Errorf("entity 1: expected #1 IFCPROJECT, got #%d %s", ent.ID, ent.Type)
	}
	if len(ent.Attrs) != 9 {
		t.Errorf("entity 1: expected 9 attrs, got %d", len(ent.Attrs))
	}

	// Second entity
	ent, err = p.Next()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ent.ID != 2 || ent.Type != "IFCWALL" {
		t.Errorf("entity 2: expected #2 IFCWALL, got #%d %s", ent.ID, ent.Type)
	}
	if len(ent.Attrs) != 8 {
		t.Errorf("entity 2: expected 8 attrs, got %d", len(ent.Attrs))
	}

	// EOF
	ent, err = p.Next()
	if err != io.EOF {
		t.Errorf("expected io.EOF, got err=%v ent=%v", err, ent)
	}
}

func TestParseEOFReturnsNilAndEOF(t *testing.T) {
	src := []byte(`#1 = IFCWALL('x');`)
	p := NewParser(src)

	_, err := p.Next()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ent, err := p.Next()
	if ent != nil {
		t.Errorf("expected nil entity at EOF, got %v", ent)
	}
	if err != io.EOF {
		t.Errorf("expected io.EOF, got %v", err)
	}
}

func TestValueKindString(t *testing.T) {
	tests := []struct {
		kind ValueKind
		want string
	}{
		{KindString, "String"},
		{KindInteger, "Integer"},
		{KindFloat, "Float"},
		{KindEnum, "Enum"},
		{KindRef, "Ref"},
		{KindList, "List"},
		{KindTyped, "Typed"},
		{KindNull, "Null"},
		{KindDerived, "Derived"},
	}
	for _, tt := range tests {
		if got := tt.kind.String(); got != tt.want {
			t.Errorf("ValueKind(%d).String() = %q, want %q", tt.kind, got, tt.want)
		}
	}
}
