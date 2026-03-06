package cli

import (
	"fmt"

	"github.com/spf13/cobra"
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
	Run: func(cmd *cobra.Command, args []string) {
		outputFormat, _ := cmd.Flags().GetString("output-format")
		header, _ := cmd.Flags().GetBool("header")
		nullValue, _ := cmd.Flags().GetString("null-value")
		file, _ := cmd.Flags().GetString("file")

		sql := ""
		if len(args) > 1 {
			sql = args[1]
		}

		fmt.Printf("query: db=%s sql=%q\n", args[0], sql)
		fmt.Printf("  output-format=%s header=%v null-value=%q file=%s\n",
			outputFormat, header, nullValue, file)
	},
}

func init() {
	f := queryCmd.Flags()
	f.String("output-format", "table", "Output format: table, csv, json, or jsonl")
	f.Bool("header", true, "Include column headers in output")
	f.String("null-value", "", "String to display for NULL values")
	f.String("file", "", "Read SQL from file instead of positional argument")

	rootCmd.AddCommand(queryCmd)
}
