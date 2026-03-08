package step

import "fmt"

// ValueKind identifies the type of a parsed STEP attribute value.
type ValueKind int

const (
	KindString  ValueKind = iota
	KindInteger
	KindFloat
	KindEnum
	KindRef
	KindList
	KindTyped // e.g. IFCLENGTHMEASURE(2.5)
	KindNull
	KindDerived
)

var valueKindNames = [...]string{
	KindString:  "String",
	KindInteger: "Integer",
	KindFloat:   "Float",
	KindEnum:    "Enum",
	KindRef:     "Ref",
	KindList:    "List",
	KindTyped:   "Typed",
	KindNull:    "Null",
	KindDerived: "Derived",
}

func (k ValueKind) String() string {
	if int(k) < len(valueKindNames) {
		return valueKindNames[k]
	}
	return fmt.Sprintf("ValueKind(%d)", int(k))
}

// StepValue represents a single parsed attribute value in a STEP entity.
type StepValue struct {
	Kind  ValueKind
	Str   string      // for KindString, KindEnum, KindTyped (type name)
	Int   int64       // for KindInteger
	Float float64     // for KindFloat
	Ref   uint64      // for KindRef
	List  []StepValue // for KindList
	Inner *StepValue  // for KindTyped (the wrapped value)
}

// Entity represents a single parsed STEP entity instance.
type Entity struct {
	ID    uint64
	Type  string
	Attrs []StepValue
}
