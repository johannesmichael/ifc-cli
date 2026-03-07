package cli

import (
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	_ "github.com/marcboeker/go-duckdb"
)

var queryCmd = &cobra.Command{
	Use:   "query <database.duckdb> [sql]",
	Short: "Run SQL against an imported DuckDB database",
	Long: `Run a SQL query against an imported DuckDB database and print results.

Pass the SQL as a positional argument or use --file to read it from a file.
Output defaults to a formatted table; use --output-format to switch to csv,
json, or jsonl for piping into other tools or for agent consumption.

Use "ifc-to-db schema" to discover available tables and columns.`,
	Example: `  # Interactive query with table output
  ifc-to-db query model.duckdb "SELECT * FROM properties LIMIT 10"

  # Run SQL from a file, output as CSV
  ifc-to-db query model.duckdb --file query.sql --output-format csv

  # Distinct property sets as JSON lines
  ifc-to-db query model.duckdb "SELECT DISTINCT pset_name FROM properties" --output-format jsonl`,
	Args: cobra.RangeArgs(1, 2),
	RunE: runQuery,
}

func runQuery(cmd *cobra.Command, args []string) error {
	dbPath := args[0]
	outputFormat, _ := cmd.Flags().GetString("output-format")
	header, _ := cmd.Flags().GetBool("header")
	nullValue, _ := cmd.Flags().GetString("null-value")
	file, _ := cmd.Flags().GetString("file")

	// Determine SQL source
	sqlText := ""
	if len(args) > 1 {
		sqlText = args[1]
	}
	if file != "" {
		data, err := os.ReadFile(file)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: reading SQL file: %s\n", err)
			os.Exit(ExitBadArguments)
		}
		sqlText = string(data)
	}
	sqlText = strings.TrimSpace(sqlText)
	if sqlText == "" {
		fmt.Fprintln(os.Stderr, "error: no SQL provided; pass as argument or use --file")
		os.Exit(ExitBadArguments)
		return nil
	}

	// Open database read-only
	db, err := sql.Open("duckdb", dbPath+"?access_mode=read_only")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: opening database: %s\n", err)
		os.Exit(ExitDatabaseError)
		return nil
	}
	defer db.Close()

	// Verify connection
	if err := db.Ping(); err != nil {
		fmt.Fprintf(os.Stderr, "error: opening database: %s\n", err)
		os.Exit(ExitDatabaseError)
		return nil
	}

	// Execute query
	rows, err := db.Query(sqlText)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: executing query: %s\n", err)
		os.Exit(ExitDatabaseError)
		return nil
	}
	defer rows.Close()

	// Get column metadata
	colTypes, err := rows.ColumnTypes()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: reading column types: %s\n", err)
		os.Exit(ExitDatabaseError)
		return nil
	}

	colNames := make([]string, len(colTypes))
	for i, ct := range colTypes {
		colNames[i] = ct.Name()
	}

	// Scan all rows into memory
	var allRows [][]interface{}
	for rows.Next() {
		vals := make([]interface{}, len(colNames))
		ptrs := make([]interface{}, len(colNames))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			fmt.Fprintf(os.Stderr, "error: scanning row: %s\n", err)
			os.Exit(ExitDatabaseError)
			return nil
		}
		allRows = append(allRows, vals)
	}
	if err := rows.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "error: reading rows: %s\n", err)
		os.Exit(ExitDatabaseError)
		return nil
	}

	// Format and output
	switch outputFormat {
	case "json":
		return writeJSON(colNames, allRows, nullValue)
	case "jsonl":
		return writeJSONL(colNames, allRows, nullValue)
	case "csv":
		return writeCSV(colNames, allRows, nullValue, header)
	default:
		return writeTable(colNames, allRows, nullValue, header, colTypes)
	}
}

// formatValue converts an interface{} value to its string representation.
func formatValue(v interface{}, nullValue string) string {
	if v == nil {
		return nullValue
	}
	switch val := v.(type) {
	case []byte:
		return string(val)
	default:
		return fmt.Sprintf("%v", val)
	}
}

// jsonValue converts a scanned value to a JSON-appropriate type.
func jsonValue(v interface{}) interface{} {
	if v == nil {
		return nil
	}
	switch val := v.(type) {
	case []byte:
		return string(val)
	default:
		return val
	}
}

// isNumeric returns true if the value is a numeric type.
func isNumeric(v interface{}) bool {
	switch v.(type) {
	case int, int8, int16, int32, int64,
		uint, uint8, uint16, uint32, uint64,
		float32, float64:
		return true
	default:
		return false
	}
}

func writeJSON(colNames []string, rows [][]interface{}, nullValue string) error {
	result := make([]map[string]interface{}, 0, len(rows))
	for _, row := range rows {
		obj := make(map[string]interface{}, len(colNames))
		for i, name := range colNames {
			obj[name] = jsonValue(row[i])
		}
		result = append(result, obj)
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(result)
}

func writeJSONL(colNames []string, rows [][]interface{}, nullValue string) error {
	enc := json.NewEncoder(os.Stdout)
	for _, row := range rows {
		obj := make(map[string]interface{}, len(colNames))
		for i, name := range colNames {
			obj[name] = jsonValue(row[i])
		}
		if err := enc.Encode(obj); err != nil {
			return err
		}
	}
	return nil
}

func writeCSV(colNames []string, rows [][]interface{}, nullValue string, showHeader bool) error {
	w := csv.NewWriter(os.Stdout)
	defer w.Flush()

	if showHeader {
		if err := w.Write(colNames); err != nil {
			return err
		}
	}
	record := make([]string, len(colNames))
	for _, row := range rows {
		for i, v := range row {
			record[i] = formatValue(v, nullValue)
		}
		if err := w.Write(record); err != nil {
			return err
		}
	}
	return nil
}

func writeTable(colNames []string, rows [][]interface{}, nullValue string, showHeader bool, colTypes []*sql.ColumnType) error {
	numCols := len(colNames)

	// Convert all values to strings and track which columns are numeric
	strRows := make([][]string, len(rows))
	colIsNumeric := make([]bool, numCols)
	// Initialize: assume numeric if there's at least one non-null value
	for i := range colIsNumeric {
		colIsNumeric[i] = true
	}

	for r, row := range rows {
		strRows[r] = make([]string, numCols)
		for c, v := range row {
			strRows[r][c] = formatValue(v, nullValue)
			if v != nil && !isNumeric(v) {
				colIsNumeric[c] = false
			}
		}
	}

	// Calculate column widths
	widths := make([]int, numCols)
	if showHeader {
		for i, name := range colNames {
			if len(name) > widths[i] {
				widths[i] = len(name)
			}
		}
	}
	for _, row := range strRows {
		for i, val := range row {
			if len(val) > widths[i] {
				widths[i] = len(val)
			}
		}
	}

	// Build format strings per column
	fmts := make([]string, numCols)
	for i, w := range widths {
		if colIsNumeric[i] {
			fmts[i] = fmt.Sprintf("%%%ds", w) // right-align
		} else {
			fmts[i] = fmt.Sprintf("%%-%ds", w) // left-align
		}
	}

	// Print header
	if showHeader {
		parts := make([]string, numCols)
		for i, name := range colNames {
			parts[i] = fmt.Sprintf(fmts[i], name)
		}
		fmt.Println(strings.Join(parts, "  "))

		// Separator line
		sep := make([]string, numCols)
		for i, w := range widths {
			sep[i] = strings.Repeat("-", w)
		}
		fmt.Println(strings.Join(sep, "  "))
	}

	// Print rows
	for _, row := range strRows {
		parts := make([]string, numCols)
		for i, val := range row {
			parts[i] = fmt.Sprintf(fmts[i], val)
		}
		fmt.Println(strings.Join(parts, "  "))
	}

	return nil
}

func init() {
	f := queryCmd.Flags()
	f.String("output-format", "table", "Output format: table, csv, json, or jsonl")
	f.Bool("header", true, "Include column headers in output")
	f.String("null-value", "", "String to display for NULL values")
	f.String("file", "", "Read SQL from file instead of positional argument")

	queryCmd.RegisterFlagCompletionFunc("output-format", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{"table", "csv", "json", "jsonl"}, cobra.ShellCompDirectiveNoFileComp
	})

	rootCmd.AddCommand(queryCmd)
}
