package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var schemaCmd = &cobra.Command{
	Use:   "schema",
	Short: "Print the DuckDB schema DDL",
	Long: `Print the DuckDB schema used by ifc-to-db, including table definitions,
column types, and constraints.

This command is designed for agent discoverability — LLM agents and scripts
can call "ifc-to-db schema --output-format json" to learn the database
structure before generating queries.

Use --tables for a quick list of table names, --table for the DDL of a
specific table, or --columns to list columns and types for a given table.`,
	Example: `  # Full schema DDL
  ifc-to-db schema

  # List table names only
  ifc-to-db schema --tables

  # DDL for a specific table
  ifc-to-db schema --table properties

  # Column names and types for a table
  ifc-to-db schema --columns properties

  # Machine-readable schema for agent consumption
  ifc-to-db schema --output-format json`,
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

	schemaCmd.RegisterFlagCompletionFunc("output-format", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{"text", "json"}, cobra.ShellCompDirectiveNoFileComp
	})

	rootCmd.AddCommand(schemaCmd)
}
