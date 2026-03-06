package step

import (
	"encoding/json"
	"strconv"
	"strings"
)

// MarshalJSON implements json.Marshaler for StepValue.
func (v StepValue) MarshalJSON() ([]byte, error) {
	return v.appendJSON(nil), nil
}

// appendJSON appends the JSON encoding of v to buf and returns the extended buffer.
func (v StepValue) appendJSON(buf []byte) []byte {
	switch v.Kind {
	case KindNull:
		return append(buf, "null"...)
	case KindDerived:
		return append(buf, `{"derived":true}`...)
	case KindString:
		return appendJSONString(buf, v.Str)
	case KindInteger:
		return strconv.AppendInt(buf, v.Int, 10)
	case KindFloat:
		return strconv.AppendFloat(buf, v.Float, 'f', -1, 64)
	case KindRef:
		buf = append(buf, `{"ref":`...)
		buf = strconv.AppendUint(buf, v.Ref, 10)
		return append(buf, '}')
	case KindEnum:
		buf = append(buf, `{"enum":`...)
		// Strip leading/trailing dots from enum value
		s := strings.Trim(v.Str, ".")
		buf = appendJSONString(buf, s)
		return append(buf, '}')
	case KindList:
		buf = append(buf, '[')
		for i, item := range v.List {
			if i > 0 {
				buf = append(buf, ',')
			}
			buf = item.appendJSON(buf)
		}
		return append(buf, ']')
	case KindTyped:
		buf = append(buf, `{"type":`...)
		buf = appendJSONString(buf, v.Str)
		buf = append(buf, `,"value":`...)
		if v.Inner != nil {
			buf = v.Inner.appendJSON(buf)
		} else {
			buf = append(buf, "null"...)
		}
		return append(buf, '}')
	default:
		return append(buf, "null"...)
	}
}

// MarshalAttrs serializes an entity's attribute list to JSON.
func MarshalAttrs(attrs []StepValue) ([]byte, error) {
	buf := make([]byte, 0, 256)
	buf = append(buf, '[')
	for i := range attrs {
		if i > 0 {
			buf = append(buf, ',')
		}
		buf = attrs[i].appendJSON(buf)
	}
	buf = append(buf, ']')
	return buf, nil
}

// appendJSONString appends a JSON-encoded string to buf.
func appendJSONString(buf []byte, s string) []byte {
	// Use json.Marshal for correctness with escaping
	b, _ := json.Marshal(s)
	return append(buf, b...)
}
