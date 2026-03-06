package step

import (
	"encoding/json"
	"testing"
)

func TestMarshalJSON_String(t *testing.T) {
	v := StepValue{Kind: KindString, Str: "hello"}
	got, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != `"hello"` {
		t.Errorf("got %s, want %q", got, `"hello"`)
	}
}

func TestMarshalJSON_StringEscaping(t *testing.T) {
	v := StepValue{Kind: KindString, Str: "line\nnew\ttab\"quote"}
	got, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	if !json.Valid(got) {
		t.Errorf("invalid JSON: %s", got)
	}
}

func TestMarshalJSON_Integer(t *testing.T) {
	v := StepValue{Kind: KindInteger, Int: 42}
	got, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "42" {
		t.Errorf("got %s, want 42", got)
	}
}

func TestMarshalJSON_Float(t *testing.T) {
	v := StepValue{Kind: KindFloat, Float: 3.14}
	got, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "3.14" {
		t.Errorf("got %s, want 3.14", got)
	}
}

func TestMarshalJSON_Enum(t *testing.T) {
	v := StepValue{Kind: KindEnum, Str: ".ELEMENT."}
	got, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != `{"enum":"ELEMENT"}` {
		t.Errorf("got %s, want %s", got, `{"enum":"ELEMENT"}`)
	}
}

func TestMarshalJSON_EnumNoDots(t *testing.T) {
	v := StepValue{Kind: KindEnum, Str: "STEEL"}
	got, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != `{"enum":"STEEL"}` {
		t.Errorf("got %s, want %s", got, `{"enum":"STEEL"}`)
	}
}

func TestMarshalJSON_Ref(t *testing.T) {
	v := StepValue{Kind: KindRef, Ref: 123}
	got, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != `{"ref":123}` {
		t.Errorf("got %s, want %s", got, `{"ref":123}`)
	}
}

func TestMarshalJSON_List(t *testing.T) {
	v := StepValue{Kind: KindList, List: []StepValue{
		{Kind: KindInteger, Int: 1},
		{Kind: KindString, Str: "two"},
	}}
	got, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != `[1,"two"]` {
		t.Errorf("got %s, want %s", got, `[1,"two"]`)
	}
}

func TestMarshalJSON_EmptyList(t *testing.T) {
	v := StepValue{Kind: KindList, List: []StepValue{}}
	got, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != `[]` {
		t.Errorf("got %s, want []", got)
	}
}

func TestMarshalJSON_NestedList(t *testing.T) {
	v := StepValue{Kind: KindList, List: []StepValue{
		{Kind: KindInteger, Int: 1},
		{Kind: KindList, List: []StepValue{
			{Kind: KindInteger, Int: 2},
			{Kind: KindInteger, Int: 3},
		}},
	}}
	got, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != `[1,[2,3]]` {
		t.Errorf("got %s, want [1,[2,3]]", got)
	}
}

func TestMarshalJSON_Typed(t *testing.T) {
	inner := StepValue{Kind: KindFloat, Float: 2.5}
	v := StepValue{Kind: KindTyped, Str: "IFCLENGTHMEASURE", Inner: &inner}
	got, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != `{"type":"IFCLENGTHMEASURE","value":2.5}` {
		t.Errorf("got %s, want %s", got, `{"type":"IFCLENGTHMEASURE","value":2.5}`)
	}
}

func TestMarshalJSON_Null(t *testing.T) {
	v := StepValue{Kind: KindNull}
	got, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "null" {
		t.Errorf("got %s, want null", got)
	}
}

func TestMarshalJSON_Derived(t *testing.T) {
	v := StepValue{Kind: KindDerived}
	got, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != `{"derived":true}` {
		t.Errorf("got %s, want %s", got, `{"derived":true}`)
	}
}

func TestMarshalAttrs(t *testing.T) {
	attrs := []StepValue{
		{Kind: KindString, Str: "Wall-01"},
		{Kind: KindNull},
		{Kind: KindRef, Ref: 42},
		{Kind: KindEnum, Str: ".ELEMENT."},
		{Kind: KindList, List: []StepValue{
			{Kind: KindRef, Ref: 10},
			{Kind: KindRef, Ref: 20},
		}},
	}
	got, err := MarshalAttrs(attrs)
	if err != nil {
		t.Fatal(err)
	}
	want := `["Wall-01",null,{"ref":42},{"enum":"ELEMENT"},[{"ref":10},{"ref":20}]]`
	if string(got) != want {
		t.Errorf("got %s\nwant %s", got, want)
	}
}

func TestMarshalJSON_Roundtrip(t *testing.T) {
	values := []StepValue{
		{Kind: KindString, Str: "test"},
		{Kind: KindInteger, Int: -99},
		{Kind: KindFloat, Float: 1.5e2},
		{Kind: KindEnum, Str: ".NOTDEFINED."},
		{Kind: KindRef, Ref: 999},
		{Kind: KindNull},
		{Kind: KindDerived},
		{Kind: KindList, List: []StepValue{
			{Kind: KindInteger, Int: 1},
			{Kind: KindList, List: []StepValue{
				{Kind: KindString, Str: "nested"},
			}},
		}},
		{Kind: KindTyped, Str: "IFCREAL", Inner: &StepValue{Kind: KindFloat, Float: 3.14}},
	}
	for i, v := range values {
		got, err := json.Marshal(v)
		if err != nil {
			t.Errorf("value %d: marshal error: %v", i, err)
			continue
		}
		if !json.Valid(got) {
			t.Errorf("value %d: produced invalid JSON: %s", i, got)
		}
	}
}

func TestMarshalAttrs_Roundtrip(t *testing.T) {
	attrs := []StepValue{
		{Kind: KindString, Str: "GlobalId-123"},
		{Kind: KindRef, Ref: 5},
		{Kind: KindNull},
		{Kind: KindTyped, Str: "IFCLABEL", Inner: &StepValue{Kind: KindString, Str: "My Wall"}},
		{Kind: KindEnum, Str: ".ELEMENT."},
	}
	got, err := MarshalAttrs(attrs)
	if err != nil {
		t.Fatal(err)
	}
	if !json.Valid(got) {
		t.Errorf("MarshalAttrs produced invalid JSON: %s", got)
	}
}

// representativeAttrs returns a typical IFC entity attribute list for benchmarking.
func representativeAttrs() []StepValue {
	return []StepValue{
		{Kind: KindString, Str: "0YvctVUKr0kugbFTf53O9L"},              // GlobalId
		{Kind: KindRef, Ref: 42},                                        // OwnerHistory
		{Kind: KindString, Str: "Basic Wall:Interior - 79mm Partition"}, // Name
		{Kind: KindNull},    // Description
		{Kind: KindDerived}, // ObjectType
		{Kind: KindRef, Ref: 197},
		{Kind: KindRef, Ref: 233},
		{Kind: KindString, Str: "6F7DCFEF-25E0-4F12-B284-6B2B7B816252"},
		{Kind: KindEnum, Str: ".ELEMENT."},
		{Kind: KindTyped, Str: "IFCLENGTHMEASURE", Inner: &StepValue{Kind: KindFloat, Float: 2500.0}},
		{Kind: KindList, List: []StepValue{
			{Kind: KindRef, Ref: 100},
			{Kind: KindRef, Ref: 101},
			{Kind: KindRef, Ref: 102},
		}},
	}
}

func BenchmarkMarshalAttrs(b *testing.B) {
	attrs := representativeAttrs()
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_, _ = MarshalAttrs(attrs)
	}
}

func BenchmarkMarshalJSON(b *testing.B) {
	attrs := representativeAttrs()
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		for j := range attrs {
			_, _ = json.Marshal(attrs[j])
		}
	}
}
