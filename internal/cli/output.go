package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
)

// ImportResult holds the structured result of an IFC import operation.
type ImportResult struct {
	Status          string           `json:"status"`
	InputFile       string           `json:"input_file"`
	OutputFile      string           `json:"output_file"`
	SchemaVersion   string           `json:"schema_version,omitempty"`
	DurationMs      int64            `json:"duration_ms"`
	EntitiesParsed  int64            `json:"entities_parsed"`
	EntitiesErrored int64            `json:"entities_errored"`
	TablesPopulated []string         `json:"tables_populated"`
	RowCounts       map[string]int64 `json:"row_counts"`
	Phases          []PhaseResult    `json:"phases"`
	Warnings        []string         `json:"warnings,omitempty"`
	Errors          []string         `json:"errors,omitempty"`
}

// PhaseResult holds timing and status for a single import phase.
type PhaseResult struct {
	Name       string `json:"name"`
	DurationMs int64  `json:"duration_ms"`
	Status     string `json:"status"`
}

// WriteOutput writes data to w in the specified format ("json" or "text").
func WriteOutput(w io.Writer, format string, data any) error {
	if format == "json" {
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(data)
	}
	return writeText(w, data)
}

func writeText(w io.Writer, data any) error {
	r, ok := data.(*ImportResult)
	if !ok {
		// Generic fallback: marshal to JSON then print.
		b, err := json.MarshalIndent(data, "", "  ")
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(w, string(b))
		return err
	}

	lines := []string{
		fmt.Sprintf("Status:           %s", r.Status),
		fmt.Sprintf("Input:            %s", r.InputFile),
		fmt.Sprintf("Output:           %s", r.OutputFile),
	}
	if r.SchemaVersion != "" {
		lines = append(lines, fmt.Sprintf("Schema:           %s", r.SchemaVersion))
	}
	lines = append(lines,
		fmt.Sprintf("Duration:         %d ms", r.DurationMs),
		fmt.Sprintf("Entities parsed:  %d", r.EntitiesParsed),
		fmt.Sprintf("Entities errored: %d", r.EntitiesErrored),
	)

	if len(r.TablesPopulated) > 0 {
		lines = append(lines, fmt.Sprintf("Tables:           %s", strings.Join(r.TablesPopulated, ", ")))
	}

	if len(r.RowCounts) > 0 {
		lines = append(lines, "Row counts:")
		keys := make([]string, 0, len(r.RowCounts))
		for k := range r.RowCounts {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			lines = append(lines, fmt.Sprintf("  %-20s %d", k, r.RowCounts[k]))
		}
	}

	if len(r.Phases) > 0 {
		lines = append(lines, "Phases:")
		for _, p := range r.Phases {
			lines = append(lines, fmt.Sprintf("  %-20s %d ms  %s", p.Name, p.DurationMs, p.Status))
		}
	}

	if len(r.Warnings) > 0 {
		lines = append(lines, "Warnings:")
		for _, w := range r.Warnings {
			lines = append(lines, fmt.Sprintf("  - %s", w))
		}
	}

	if len(r.Errors) > 0 {
		lines = append(lines, "Errors:")
		for _, e := range r.Errors {
			lines = append(lines, fmt.Sprintf("  - %s", e))
		}
	}

	_, err := fmt.Fprintln(w, strings.Join(lines, "\n"))
	return err
}
