package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var schemaCmd = &cobra.Command{
	Use:   "schema",
	Short: "Print the DuckDB schema DDL",
	Run: func(cmd *cobra.Command, args []string) {
		table, _ := cmd.Flags().GetString("table")
		tables, _ := cmd.Flags().GetBool("tables")
		columns, _ := cmd.Flags().GetString("columns")
		outputFormat, _ := cmd.Flags().GetString("output-format")

		fmt.Printf("schema: table=%s tables=%v columns=%s output-format=%s\n",
			table, tables, columns, outputFormat)
	},
}

func init() {
	f := schemaCmd.Flags()
	f.String("table", "", "Print DDL for a specific table")
	f.Bool("tables", false, "List table names only")
	f.String("columns", "", "List columns for a specific table")
	f.String("output-format", "text", "Output format: text or json")

	rootCmd.AddCommand(schemaCmd)
}
