package cli

import (
	"encoding/json"
	"fmt"
)

// ParseError represents an error encountered during IFC file parsing.
type ParseError struct {
	File     string `json:"file"`
	Position int    `json:"position"`
	Line     int    `json:"line"`
	Message  string `json:"message"`
}

func (e *ParseError) Error() string {
	return fmt.Sprintf("%s:%d (pos %d): %s", e.File, e.Line, e.Position, e.Message)
}

func (e *ParseError) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Type     string `json:"type"`
		File     string `json:"file"`
		Position int    `json:"position"`
		Line     int    `json:"line"`
		Message  string `json:"message"`
	}{
		Type:     "parse_error",
		File:     e.File,
		Position: e.Position,
		Line:     e.Line,
		Message:  e.Message,
	})
}

// EntityError represents an error related to a specific IFC entity.
type EntityError struct {
	EntityID uint64 `json:"entity_id"`
	Phase    string `json:"phase"`
	Message  string `json:"message"`
}

func (e *EntityError) Error() string {
	return fmt.Sprintf("entity #%d [%s]: %s", e.EntityID, e.Phase, e.Message)
}

func (e *EntityError) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Type     string `json:"type"`
		EntityID uint64 `json:"entity_id"`
		Phase    string `json:"phase"`
		Message  string `json:"message"`
	}{
		Type:     "entity_error",
		EntityID: e.EntityID,
		Phase:    e.Phase,
		Message:  e.Message,
	})
}

// DatabaseError represents a DuckDB operation failure.
type DatabaseError struct {
	Operation string `json:"operation"`
	Message   string `json:"message"`
}

func (e *DatabaseError) Error() string {
	return fmt.Sprintf("database %s: %s", e.Operation, e.Message)
}

func (e *DatabaseError) MarshalJSON() ([]byte, error) {
	return json.Marshal(struct {
		Type      string `json:"type"`
		Operation string `json:"operation"`
		Message   string `json:"message"`
	}{
		Type:      "database_error",
		Operation: e.Operation,
		Message:   e.Message,
	})
}

// FormatError formats an error as text or JSON depending on the format string.
func FormatError(err error, format string) string {
	if format == "json" {
		if m, ok := err.(json.Marshaler); ok {
			data, jerr := m.MarshalJSON()
			if jerr == nil {
				return string(data)
			}
		}
		// Fallback for errors that don't implement json.Marshaler.
		data, _ := json.Marshal(map[string]string{
			"type":    "error",
			"message": err.Error(),
		})
		return string(data)
	}
	return err.Error()
}
