package cli

import (
	"bufio"
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
	"ifc-cli/internal/extract"
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

If the output .duckdb file already exists, it is replaced (deleted and recreated).
Use --append to add data to an existing database instead of replacing it. This
is useful when importing multiple IFC files into one database — each import adds
its entities alongside existing data. Note that --append will fail on duplicate
entity IDs, so it is intended for importing different files, not re-importing
the same file.

Use --skip-* flags to omit specific phases, or --only to run a subset. The
--batch-size flag controls insert batching for large models.

Progress is reported to stderr. Use -q to suppress it, or --output-format json
for machine-readable output on stdout.`,
	Example: `  # Basic import (creates model.duckdb, replaces if exists)
  ifc-to-db import model.ifc

  # Custom output path, skip geometry, quiet mode
  ifc-to-db import model.ifc -o output.duckdb --skip-geometry -q

  # Import multiple files into one database
  ifc-to-db import arch.ifc -o combined.duckdb
  ifc-to-db import struct.ifc -o combined.duckdb --append

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

	appendMode, _ := cmd.Flags().GetBool("append")
	forceMode, _ := cmd.Flags().GetBool("force")

	// Default behavior: replace existing file (unless --append or --memory)
	if dbPath != "" && !appendMode {
		if _, err := os.Stat(dbPath); err == nil {
			if !forceMode {
				if !confirmReplace(dbPath, quiet) {
					return fmt.Errorf("aborted: use --force to replace without prompting, or --append to add to existing database")
				}
			}
			logger.Info("replacing existing database", "path", dbPath)
			if err := os.Remove(dbPath); err != nil {
				return fmt.Errorf("removing existing database: %w", err)
			}
		}
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
	cache := extract.NewEntityCacheEmpty()
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

		globalID := ""
		if g := db.ExtractGlobalID(entity.Attrs); g != nil {
			globalID = g.(string)
		}
		cache.Put(entity.ID, entity.Type, globalID, entity.Attrs)

		entityCount++
		progress.Update(parser.ByteOffset(), entityCount)
	}

done:
	// Flush and close writer
	if err := writer.Close(); err != nil {
		return fmt.Errorf("closing writer: %w", err)
	}

	// Finish parse progress before post-processing phases start printing.
	stats := parser.Stats()
	progress.Finish(stats.TotalEntities, stats.ErrorCount)

	// Post-processing: extract denormalized tables
	skipProperties, _ := cmd.Flags().GetBool("skip-properties")
	skipRelationships, _ := cmd.Flags().GetBool("skip-relationships")
	skipGeometry, _ := cmd.Flags().GetBool("skip-geometry")
	only, _ := cmd.Flags().GetStringSlice("only")

	// If --only is set, skip everything not listed
	onlySet := make(map[string]bool)
	for _, o := range only {
		onlySet[strings.ToLower(strings.TrimSpace(o))] = true
	}
	useOnly := len(onlySet) > 0

	shouldRun := func(phase string, skip bool) bool {
		if skip {
			return false
		}
		if useOnly {
			return onlySet[phase]
		}
		return true
	}

	// Build entity cache: use the in-memory cache from parsing if available,
	// otherwise fall back to loading from DB (e.g., re-extraction of existing DB).
	if cache.Len() == 0 {
		if shouldRun("properties", skipProperties) || shouldRun("relationships", skipRelationships) || shouldRun("spatial", skipRelationships) || shouldRun("geometry", skipGeometry) {
			phaseStart := time.Now()
			if !quiet {
				fmt.Fprintf(os.Stderr, "\r\033[K  Loading entity cache...")
			}
			var err error
			cache, err = extract.NewEntityCache(database.DB)
			if err != nil {
				logger.Error("building entity cache", "error", err)
			} else if !quiet {
				fmt.Fprintf(os.Stderr, "\r\033[K  Loading entity cache... done (%s)\n", formatDuration(time.Since(phaseStart)))
			}
		}
	}

	if shouldRun("properties", skipProperties) && cache.Len() > 0 {
		phaseStart := time.Now()
		lastProgress := time.Time{}
		if !quiet {
			fmt.Fprintf(os.Stderr, "  Extracting properties...")
		}
		progressFn := func(detail string, count int) {
			if quiet {
				return
			}
			now := time.Now()
			if now.Sub(lastProgress) < 100*time.Millisecond {
				return
			}
			lastProgress = now
			fmt.Fprintf(os.Stderr, "\r\033[K  Extracting properties... %s %s", formatCount(count), detail)
		}
		if err := extract.ExtractProperties(database.DB, cache, progressFn); err != nil {
			logger.Error("extracting properties", "error", err)
		} else if !quiet {
			fmt.Fprintf(os.Stderr, "\r\033[K  Extracting properties... done (%s)\n", formatDuration(time.Since(phaseStart)))
		}
	}

	if shouldRun("relationships", skipRelationships) && cache.Len() > 0 {
		phaseStart := time.Now()
		lastProgress := time.Time{}
		if !quiet {
			fmt.Fprintf(os.Stderr, "  Extracting relationships...")
		}
		progressFn := func(detail string, count int) {
			if quiet {
				return
			}
			now := time.Now()
			if now.Sub(lastProgress) < 100*time.Millisecond {
				return
			}
			lastProgress = now
			fmt.Fprintf(os.Stderr, "\r\033[K  Extracting relationships... %s %s", formatCount(count), detail)
		}
		if err := extract.ExtractRelationships(database.DB, cache, progressFn); err != nil {
			logger.Error("extracting relationships", "error", err)
		} else if !quiet {
			fmt.Fprintf(os.Stderr, "\r\033[K  Extracting relationships... done (%s)\n", formatDuration(time.Since(phaseStart)))
		}
	}

	if shouldRun("spatial", skipRelationships) && cache.Len() > 0 {
		phaseStart := time.Now()
		lastProgress := time.Time{}
		if !quiet {
			fmt.Fprintf(os.Stderr, "  Extracting spatial hierarchy...")
		}
		progressFn := func(detail string, count int) {
			if quiet {
				return
			}
			now := time.Now()
			if now.Sub(lastProgress) < 100*time.Millisecond {
				return
			}
			lastProgress = now
			fmt.Fprintf(os.Stderr, "\r\033[K  Extracting spatial hierarchy... %s %s", formatCount(count), detail)
		}
		if err := extract.ExtractSpatialHierarchy(database.DB, cache, progressFn); err != nil {
			logger.Error("extracting spatial hierarchy", "error", err)
		} else if !quiet {
			fmt.Fprintf(os.Stderr, "\r\033[K  Extracting spatial hierarchy... done (%s)\n", formatDuration(time.Since(phaseStart)))
		}
	}

	if shouldRun("geometry", skipGeometry) && cache.Len() > 0 {
		phaseStart := time.Now()
		lastProgress := time.Time{}
		if !quiet {
			fmt.Fprintf(os.Stderr, "  Extracting geometry...")
		}
		progressFn := func(detail string, count int) {
			if quiet {
				return
			}
			now := time.Now()
			if now.Sub(lastProgress) < 100*time.Millisecond {
				return
			}
			lastProgress = now
			fmt.Fprintf(os.Stderr, "\r\033[K  Extracting geometry... %s %s", formatCount(count), detail)
		}
		if err := extract.ExtractGeometry(database.DB, cache, progressFn); err != nil {
			logger.Error("extracting geometry", "error", err)
		} else if !quiet {
			fmt.Fprintf(os.Stderr, "\r\033[K  Extracting geometry... done (%s)\n", formatDuration(time.Since(phaseStart)))
		}
	}

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

// confirmReplace asks the user to confirm replacing an existing file.
// Returns true if user says yes, false otherwise.
func confirmReplace(path string, quiet bool) bool {
	if quiet {
		return false
	}
	// Check if stdin is a terminal
	fi, err := os.Stdin.Stat()
	if err != nil || (fi.Mode()&os.ModeCharDevice) == 0 {
		// Not a terminal (piped input) — don't prompt, abort
		fmt.Fprintf(os.Stderr, "Output file %s already exists. Use --force to replace or --append to add data.\n", path)
		return false
	}

	fmt.Fprintf(os.Stderr, "Output file %s already exists. Replace? [y/N] ", path)
	scanner := bufio.NewScanner(os.Stdin)
	if scanner.Scan() {
		answer := strings.TrimSpace(strings.ToLower(scanner.Text()))
		return answer == "y" || answer == "yes"
	}
	return false
}

// formatCount formats an integer with comma separators for readability.
func formatCount(n int) string {
	if n < 1000 {
		return fmt.Sprintf("%d", n)
	}
	if n < 1000000 {
		return fmt.Sprintf("%d,%03d", n/1000, n%1000)
	}
	return fmt.Sprintf("%d,%03d,%03d", n/1000000, (n/1000)%1000, n%1000)
}

// formatDuration formats a duration as a human-readable string.
func formatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	return fmt.Sprintf("%.1fs", d.Seconds())
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
	f.BoolP("force", "f", false, "Replace existing database without prompting")
	f.Int("batch-size", 10000, "Number of entities per batch insert")
	f.BoolP("quiet", "q", false, "Suppress progress output")
	f.BoolP("verbose", "v", false, "Detailed logging")
	f.String("log-file", "", "Write log output to file")
	f.String("output-format", "text", "Output format: text or json")

	importCmd.RegisterFlagCompletionFunc("output-format", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{"text", "json"}, cobra.ShellCompDirectiveNoFileComp
	})

	rootCmd.AddCommand(importCmd)
}
