package cli

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"ifc-cli/internal/db"
	"ifc-cli/internal/step"
)

// InfoResult holds the structured result of an info inspection.
type InfoResult struct {
	File            string            `json:"file"`
	FileType        string            `json:"file_type"`
	Schema          string            `json:"schema,omitempty"`
	OriginSystem    string            `json:"originating_system,omitempty"`
	Preprocessor    string            `json:"preprocessor,omitempty"`
	Description     string            `json:"description,omitempty"`
	EntityCount     int64             `json:"entity_count,omitempty"`
	TopEntityTypes  []EntityTypeCount `json:"top_entity_types,omitempty"`
	Metadata        map[string]string `json:"metadata,omitempty"`
	TableRowCounts  map[string]int64  `json:"table_row_counts,omitempty"`
}

// EntityTypeCount holds a type name and its count for the histogram.
type EntityTypeCount struct {
	Type  string `json:"type"`
	Count int64  `json:"count"`
}

var infoCmd = &cobra.Command{
	Use:   "info <file>",
	Short: "Quick inspection of an IFC or DuckDB file",
	Long: `Quickly inspect an IFC file's header metadata or a DuckDB database's contents.

For IFC files: displays the schema version, originating system, preprocessor,
file description, entity count, and top 10 entity types by frequency.

For DuckDB files: displays stored file_metadata and row counts per table.

Use this to verify a file before importing or to check which IFC version a
model uses.`,
	Example: `  # Inspect IFC file metadata
  ifc-to-db info model.ifc

  # Inspect a DuckDB database
  ifc-to-db info model.duckdb

  # Machine-readable output
  ifc-to-db info model.ifc --output-format json`,
	Args: cobra.ExactArgs(1),
	RunE: runInfo,
}

func runInfo(cmd *cobra.Command, args []string) error {
	filePath := args[0]
	outputFormat, _ := cmd.Flags().GetString("output-format")

	if strings.HasSuffix(strings.ToLower(filePath), ".duckdb") {
		return runInfoDuckDB(filePath, outputFormat)
	}
	return runInfoIFC(filePath, outputFormat)
}

func runInfoIFC(filePath, outputFormat string) error {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("reading file: %w", err)
	}

	parser := step.NewParser(data)
	meta := parser.Metadata()

	// Count entities and build type histogram
	typeCounts := make(map[string]int64)
	var totalCount int64
	for {
		entity, err := parser.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue
		}
		totalCount++
		typeCounts[entity.Type]++
	}

	// Build top 10 entity types
	type kv struct {
		key   string
		count int64
	}
	sorted := make([]kv, 0, len(typeCounts))
	for k, v := range typeCounts {
		sorted = append(sorted, kv{k, v})
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].count > sorted[j].count
	})
	limit := 10
	if len(sorted) < limit {
		limit = len(sorted)
	}
	topTypes := make([]EntityTypeCount, limit)
	for i := 0; i < limit; i++ {
		topTypes[i] = EntityTypeCount{Type: sorted[i].key, Count: sorted[i].count}
	}

	// Re-read metadata after full parse
	meta = parser.Metadata()

	result := &InfoResult{
		File:           filePath,
		FileType:       "ifc",
		EntityCount:    totalCount,
		TopEntityTypes: topTypes,
	}
	if meta != nil {
		if len(meta.SchemaIdentifiers) > 0 {
			result.Schema = strings.Join(meta.SchemaIdentifiers, ", ")
		}
		result.OriginSystem = meta.OriginatingSystem
		result.Preprocessor = meta.Preprocessor
		result.Description = meta.Description
	}

	if outputFormat == "json" {
		return WriteOutput(os.Stdout, outputFormat, result)
	}
	return writeInfoText(os.Stdout, result)
}

func runInfoDuckDB(filePath, outputFormat string) error {
	database, err := db.Open(filePath)
	if err != nil {
		return fmt.Errorf("opening database: %w", err)
	}
	defer database.Close()

	result := &InfoResult{
		File:           filePath,
		FileType:       "duckdb",
		Metadata:       make(map[string]string),
		TableRowCounts: make(map[string]int64),
	}

	// Query file_metadata table
	rows, err := database.DB.Query("SELECT key, value FROM file_metadata ORDER BY key")
	if err == nil {
		defer rows.Close()
		for rows.Next() {
			var k, v string
			if err := rows.Scan(&k, &v); err == nil {
				result.Metadata[k] = v
			}
		}
	}

	// Query row counts per table
	tableRows, err := database.DB.Query("SELECT table_name, estimated_row_count FROM duckdb_tables() WHERE schema_name='main'")
	if err == nil {
		defer tableRows.Close()
		for tableRows.Next() {
			var name string
			var count int64
			if err := tableRows.Scan(&name, &count); err == nil {
				result.TableRowCounts[name] = count
			}
		}
	}

	if outputFormat == "json" {
		return WriteOutput(os.Stdout, outputFormat, result)
	}
	return writeInfoText(os.Stdout, result)
}

func writeInfoText(w io.Writer, r *InfoResult) error {
	var lines []string

	lines = append(lines, fmt.Sprintf("File:               %s", r.File))
	lines = append(lines, fmt.Sprintf("Type:               %s", r.FileType))

	if r.FileType == "ifc" {
		if r.Schema != "" {
			lines = append(lines, fmt.Sprintf("Schema:             %s", r.Schema))
		}
		if r.OriginSystem != "" {
			lines = append(lines, fmt.Sprintf("Originating system: %s", r.OriginSystem))
		}
		if r.Preprocessor != "" {
			lines = append(lines, fmt.Sprintf("Preprocessor:       %s", r.Preprocessor))
		}
		if r.Description != "" {
			lines = append(lines, fmt.Sprintf("Description:        %s", r.Description))
		}
		lines = append(lines, fmt.Sprintf("Entity count:       %d", r.EntityCount))

		if len(r.TopEntityTypes) > 0 {
			lines = append(lines, "Top entity types:")
			for _, et := range r.TopEntityTypes {
				lines = append(lines, fmt.Sprintf("  %-40s %d", et.Type, et.Count))
			}
		}
	}

	if r.FileType == "duckdb" {
		if len(r.Metadata) > 0 {
			lines = append(lines, "Metadata:")
			keys := make([]string, 0, len(r.Metadata))
			for k := range r.Metadata {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				lines = append(lines, fmt.Sprintf("  %-24s %s", k, r.Metadata[k]))
			}
		}

		if len(r.TableRowCounts) > 0 {
			lines = append(lines, "Tables:")
			keys := make([]string, 0, len(r.TableRowCounts))
			for k := range r.TableRowCounts {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				lines = append(lines, fmt.Sprintf("  %-24s %d rows", k, r.TableRowCounts[k]))
			}
		}
	}

	_, err := fmt.Fprintln(w, strings.Join(lines, "\n"))
	return err
}

func init() {
	infoCmd.Flags().String("output-format", "text", "Output format: text or json")

	infoCmd.RegisterFlagCompletionFunc("output-format", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{"text", "json"}, cobra.ShellCompDirectiveNoFileComp
	})

	rootCmd.AddCommand(infoCmd)
}
