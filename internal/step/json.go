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

// UnmarshalAttrs parses a JSON attribute array (as produced by MarshalAttrs)
// back into []StepValue.
func UnmarshalAttrs(data []byte) ([]StepValue, error) {
	var raw []json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	vals := make([]StepValue, len(raw))
	for i, r := range raw {
		v, err := unmarshalStepValue(r)
		if err != nil {
			return nil, err
		}
		vals[i] = v
	}
	return vals, nil
}

// unmarshalStepValue converts a single JSON-encoded attribute back to a StepValue.
func unmarshalStepValue(data json.RawMessage) (StepValue, error) {
	s := strings.TrimSpace(string(data))
	if s == "null" {
		return StepValue{Kind: KindNull}, nil
	}

	// Try object: could be ref, enum, typed, or derived
	if len(s) > 0 && s[0] == '{' {
		var obj map[string]json.RawMessage
		if err := json.Unmarshal(data, &obj); err != nil {
			return StepValue{}, err
		}
		if _, ok := obj["derived"]; ok {
			return StepValue{Kind: KindDerived}, nil
		}
		if raw, ok := obj["ref"]; ok {
			var ref uint64
			if err := json.Unmarshal(raw, &ref); err != nil {
				return StepValue{}, err
			}
			return StepValue{Kind: KindRef, Ref: ref}, nil
		}
		if raw, ok := obj["enum"]; ok {
			var e string
			if err := json.Unmarshal(raw, &e); err != nil {
				return StepValue{}, err
			}
			return StepValue{Kind: KindEnum, Str: "." + e + "."}, nil
		}
		if rawType, ok := obj["type"]; ok {
			var typeName string
			if err := json.Unmarshal(rawType, &typeName); err != nil {
				return StepValue{}, err
			}
			var inner StepValue
			if rawVal, vok := obj["value"]; vok {
				var err error
				inner, err = unmarshalStepValue(rawVal)
				if err != nil {
					return StepValue{}, err
				}
			}
			return StepValue{Kind: KindTyped, Str: typeName, Inner: &inner}, nil
		}
		return StepValue{Kind: KindNull}, nil
	}

	// Try array → KindList
	if len(s) > 0 && s[0] == '[' {
		var items []json.RawMessage
		if err := json.Unmarshal(data, &items); err != nil {
			return StepValue{}, err
		}
		list := make([]StepValue, len(items))
		for i, item := range items {
			v, err := unmarshalStepValue(item)
			if err != nil {
				return StepValue{}, err
			}
			list[i] = v
		}
		return StepValue{Kind: KindList, List: list}, nil
	}

	// Try string
	if len(s) > 0 && s[0] == '"' {
		var str string
		if err := json.Unmarshal(data, &str); err != nil {
			return StepValue{}, err
		}
		return StepValue{Kind: KindString, Str: str}, nil
	}

	// Try number: integer if no '.', float otherwise
	if strings.ContainsAny(s, ".eE") {
		var f float64
		if err := json.Unmarshal(data, &f); err != nil {
			return StepValue{}, err
		}
		return StepValue{Kind: KindFloat, Float: f}, nil
	}
	var n int64
	if err := json.Unmarshal(data, &n); err != nil {
		return StepValue{}, err
	}
	return StepValue{Kind: KindInteger, Int: n}, nil
}
