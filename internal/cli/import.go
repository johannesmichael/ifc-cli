package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"ifc-cli/internal/db"
	"ifc-cli/internal/step"
)

var importCmd = &cobra.Command{
	Use:   "import <file.ifc>",
	Short: "Parse an IFC file and write contents to DuckDB",
	Long: `Parse an IFC STEP file and write its contents into a DuckDB database.

The import command streams through the IFC file, extracts entities, properties,
relationships, quantities, and spatial hierarchy, then writes them into typed
DuckDB tables for SQL analysis.

By default the output file is named after the input (model.ifc → model.duckdb).
Use --output to specify a different path, or --memory for an in-memory database
that is discarded after the command exits (useful for piping into query).

Use --skip-* flags to omit specific phases, or --only to run a subset. The
--batch-size flag controls insert batching for large models.

Progress is reported to stderr. Use -q to suppress it, or --output-format json
for machine-readable output on stdout.`,
	Example: `  # Basic import (creates model.duckdb)
  ifc-to-db import model.ifc

  # Custom output path, skip geometry, quiet mode
  ifc-to-db import model.ifc -o output.duckdb --skip-geometry -q

  # JSON output, only properties and spatial phases
  ifc-to-db import model.ifc --output-format json --only properties,spatial

  # Verbose logging to a file
  ifc-to-db import model.ifc -v --log-file import.log`,
	Args: cobra.ExactArgs(1),
	RunE: runImport,
}

func runImport(cmd *cobra.Command, args []string) error {
	filePath := args[0]

	// Read flags
	outputPath, _ := cmd.Flags().GetString("output")
	memory, _ := cmd.Flags().GetBool("memory")
	batchSize, _ := cmd.Flags().GetInt("batch-size")
	quiet, _ := cmd.Flags().GetBool("quiet")
	verbose, _ := cmd.Flags().GetBool("verbose")
	logFile, _ := cmd.Flags().GetString("log-file")
	outputFormat, _ := cmd.Flags().GetString("output-format")

	jsonOutput := outputFormat == "json"

	// Default output path
	if outputPath == "" && !memory {
		outputPath = strings.TrimSuffix(filePath, filepath.Ext(filePath)) + ".duckdb"
	}

	// Setup logging
	logger := SetupLogging(verbose, quiet, logFile, jsonOutput)

	// Graceful shutdown via SIGINT
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	// Read input file
	logger.Info("reading input file", "path", filePath)
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("reading input file: %w", err)
	}

	startTime := time.Now()

	// Open database
	dbPath := outputPath
	if memory {
		dbPath = ""
	}
	logger.Info("opening database", "path", dbPath, "memory", memory)
	database, err := db.Open(dbPath)
	if err != nil {
		return fmt.Errorf("opening database: %w", err)
	}
	defer database.Close()

	// Create parser
	parser := step.NewParser(data)

	// Create writer
	writer, err := db.NewWriter(database, batchSize)
	if err != nil {
		return fmt.Errorf("creating writer: %w", err)
	}

	// Create progress reporter
	progress := NewProgressReporter(os.Stderr, int64(len(data)), quiet, jsonOutput)

	// Parse and write loop
	var entityCount int64
	logger.Info("starting parse loop")
	for {
		// Check for cancellation
		select {
		case <-ctx.Done():
			logger.Warn("interrupted, flushing remaining entities")
			goto done
		default:
		}

		entity, err := parser.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			logger.Error("parse error", "error", err)
			continue
		}

		if err := writer.Write(entity); err != nil {
			return fmt.Errorf("writing entity #%d: %w", entity.ID, err)
		}
		entityCount++
		progress.Update(parser.ByteOffset(), entityCount)
	}

done:
	// Flush and close writer
	if err := writer.Close(); err != nil {
		return fmt.Errorf("closing writer: %w", err)
	}

	stats := parser.Stats()
	progress.Finish(stats.TotalEntities, stats.ErrorCount)

	// Write metadata
	extra := map[string]string{
		"source_file":   filePath,
		"import_time":   time.Now().Format(time.RFC3339),
		"entity_count":  fmt.Sprintf("%d", stats.TotalEntities),
		"error_count":   fmt.Sprintf("%d", stats.ErrorCount),
		"parser_version": Version,
	}
	if err := db.WriteMetadata(database.DB, parser.Metadata(), extra); err != nil {
		logger.Error("writing metadata", "error", err)
	}

	duration := time.Since(startTime)

	// Build result
	outputFile := outputPath
	if memory {
		outputFile = ":memory:"
	}
	schemaVersion := ""
	if meta := parser.Metadata(); meta != nil && len(meta.SchemaIdentifiers) > 0 {
		schemaVersion = strings.Join(meta.SchemaIdentifiers, ", ")
	}

	result := &ImportResult{
		Status:          "ok",
		InputFile:       filePath,
		OutputFile:      outputFile,
		SchemaVersion:   schemaVersion,
		DurationMs:      duration.Milliseconds(),
		EntitiesParsed:  stats.TotalEntities,
		EntitiesErrored: stats.ErrorCount,
		TablesPopulated: []string{"entities", "file_metadata"},
		RowCounts:       map[string]int64{"entities": stats.TotalEntities},
	}

	if ctx.Err() != nil {
		result.Status = "interrupted"
		result.Warnings = append(result.Warnings, "import interrupted by user")
	}

	if stats.ErrorCount > 0 {
		result.Warnings = append(result.Warnings, fmt.Sprintf("%d entities had parse errors", stats.ErrorCount))
	}

	return WriteOutput(os.Stdout, outputFormat, result)
}

func init() {
	f := importCmd.Flags()
	f.StringP("output", "o", "", "Output DuckDB file path (default: <input_name>.duckdb)")
	f.Bool("memory", false, "Use in-memory DuckDB (no file output)")
	f.Bool("skip-properties", false, "Skip property set denormalization")
	f.Bool("skip-geometry", false, "Skip geometry serialization")
	f.Bool("skip-relationships", false, "Skip relationship extraction")
	f.Bool("skip-quantities", false, "Skip quantity extraction")
	f.StringSlice("only", nil, "Run only specified phases (properties, quantities, relationships, spatial, geometry)")
	f.Bool("append", false, "Append to existing database instead of overwriting")
	f.Int("batch-size", 10000, "Number of entities per batch insert")
	f.BoolP("quiet", "q", false, "Suppress progress output")
	f.BoolP("verbose", "v", false, "Detailed logging")
	f.String("log-file", "", "Write log output to file")
	f.String("output-format", "text", "Output format: text or json")

	rootCmd.AddCommand(importCmd)
}
