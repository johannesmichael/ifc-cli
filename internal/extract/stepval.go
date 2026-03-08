package extract

import (
	"fmt"
	"strconv"
	"strings"

	"ifc-cli/internal/step"
)

// StepRef returns the ref ID if v is a KindRef.
func StepRef(v step.StepValue) (uint64, bool) {
	if v.Kind == step.KindRef {
		return v.Ref, true
	}
	return 0, false
}

// StepRefList collects all ref IDs from a KindList value.
func StepRefList(v step.StepValue) []uint64 {
	if v.Kind != step.KindList {
		return nil
	}
	var refs []uint64
	for _, item := range v.List {
		if item.Kind == step.KindRef {
			refs = append(refs, item.Ref)
		}
	}
	return refs
}

// StepString returns the string if v is a KindString.
func StepString(v step.StepValue) (string, bool) {
	if v.Kind == step.KindString {
		return v.Str, true
	}
	return "", false
}

// StepFloat returns the float value, handling KindFloat, KindInteger, and KindTyped wrapping a number.
func StepFloat(v step.StepValue) float64 {
	switch v.Kind {
	case step.KindFloat:
		return v.Float
	case step.KindInteger:
		return float64(v.Int)
	case step.KindTyped:
		if v.Inner != nil {
			return StepFloat(*v.Inner)
		}
	}
	return 0
}

// StepEnum returns the enum string (with dots stripped) if v is a KindEnum.
func StepEnum(v step.StepValue) (string, bool) {
	if v.Kind == step.KindEnum {
		return strings.Trim(v.Str, "."), true
	}
	return "", false
}

// StepFormatValue formats a StepValue as a display string (parallels formatAttrValue).
func StepFormatValue(v step.StepValue) string {
	switch v.Kind {
	case step.KindNull, step.KindDerived:
		return ""
	case step.KindString:
		return v.Str
	case step.KindInteger:
		return strconv.FormatInt(v.Int, 10)
	case step.KindFloat:
		return strconv.FormatFloat(v.Float, 'f', -1, 64)
	case step.KindEnum:
		s := strings.Trim(v.Str, ".")
		if s == "T" {
			return "true"
		}
		if s == "F" {
			return "false"
		}
		return s
	case step.KindRef:
		return fmt.Sprintf("#%d", v.Ref)
	case step.KindTyped:
		if v.Inner != nil {
			return StepFormatValue(*v.Inner)
		}
		return ""
	case step.KindList:
		vals := make([]string, 0, len(v.List))
		for _, item := range v.List {
			s := StepFormatValue(item)
			if s != "" {
				vals = append(vals, s)
			}
		}
		return strings.Join(vals, ", ")
	}
	return ""
}

// StepFormatValueType returns the type name for typed values (parallels formatAttrValueType).
func StepFormatValueType(v step.StepValue) string {
	if v.Kind == step.KindTyped {
		return v.Str
	}
	return ""
}
