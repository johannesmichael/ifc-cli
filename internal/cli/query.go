package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var queryCmd = &cobra.Command{
	Use:   "query <database.duckdb> [sql]",
	Short: "Run SQL against an imported DuckDB database",
	Args:  cobra.RangeArgs(1, 2),
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
