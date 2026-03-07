package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"ifc-cli/internal/db"
)

// SchemaResult holds structured schema output for JSON serialization.
type SchemaResult struct {
	Tables  []string            `json:"tables,omitempty"`
	DDL     string              `json:"ddl,omitempty"`
	Columns []SchemaColumnInfo  `json:"columns,omitempty"`
}

// SchemaColumnInfo describes a single column.
type SchemaColumnInfo struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Nullable bool   `json:"nullable"`
}

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
	RunE: runSchema,
}

func runSchema(cmd *cobra.Command, args []string) error {
	tableName, _ := cmd.Flags().GetString("table")
	listTables, _ := cmd.Flags().GetBool("tables")
	columnsFor, _ := cmd.Flags().GetString("columns")
	outputFormat, _ := cmd.Flags().GetString("output-format")

	switch {
	case listTables:
		return schemaListTables(outputFormat)
	case columnsFor != "":
		return schemaColumns(columnsFor, outputFormat)
	case tableName != "":
		return schemaTableDDL(tableName, outputFormat)
	default:
		return schemaFullDDL(outputFormat)
	}
}

// schemaFullDDL prints all DDL statements concatenated.
func schemaFullDDL(format string) error {
	ddl := strings.Join(db.DDLStatements(), ";\n") + ";"
	if format == "json" {
		return WriteOutput(os.Stdout, format, &SchemaResult{DDL: ddl})
	}
	fmt.Println(ddl)
	return nil
}

// schemaTableDDL prints DDL for a specific table.
func schemaTableDDL(name, format string) error {
	// Find DDL statements that reference this table name.
	var matched []string
	for _, stmt := range db.DDLStatements() {
		lower := strings.ToLower(stmt)
		// Match CREATE TABLE <name> or CREATE INDEX ... ON <name>(
		if strings.Contains(lower, "table if not exists "+name) ||
			strings.Contains(lower, "table "+name) ||
			strings.Contains(lower, " on "+name+"(") {
			matched = append(matched, stmt)
		}
	}
	if len(matched) == 0 {
		return fmt.Errorf("unknown table: %s", name)
	}
	ddl := strings.Join(matched, ";\n") + ";"
	if format == "json" {
		return WriteOutput(os.Stdout, format, &SchemaResult{DDL: ddl})
	}
	fmt.Println(ddl)
	return nil
}

// schemaListTables queries information_schema for table names.
func schemaListTables(format string) error {
	database, err := openSchemaDB()
	if err != nil {
		return err
	}
	defer database.Close()

	rows, err := database.DB.Query(
		`SELECT table_name FROM information_schema.tables WHERE table_schema='main' ORDER BY table_name`)
	if err != nil {
		return fmt.Errorf("querying tables: %w", err)
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			return err
		}
		tables = append(tables, t)
	}
	if err := rows.Err(); err != nil {
		return err
	}

	if format == "json" {
		return WriteOutput(os.Stdout, format, &SchemaResult{Tables: tables})
	}
	for _, t := range tables {
		fmt.Println(t)
	}
	return nil
}

// schemaColumns queries information_schema for column details of a table.
func schemaColumns(name, format string) error {
	database, err := openSchemaDB()
	if err != nil {
		return err
	}
	defer database.Close()

	// Verify table exists.
	var count int
	if err := database.DB.QueryRow(
		`SELECT count(*) FROM information_schema.tables WHERE table_schema='main' AND table_name=?`, name).Scan(&count); err != nil {
		return err
	}
	if count == 0 {
		return fmt.Errorf("unknown table: %s", name)
	}

	rows, err := database.DB.Query(
		`SELECT column_name, data_type, is_nullable FROM information_schema.columns WHERE table_name=? ORDER BY ordinal_position`, name)
	if err != nil {
		return fmt.Errorf("querying columns: %w", err)
	}
	defer rows.Close()

	var cols []SchemaColumnInfo
	for rows.Next() {
		var cname, ctype, nullable string
		if err := rows.Scan(&cname, &ctype, &nullable); err != nil {
			return err
		}
		cols = append(cols, SchemaColumnInfo{
			Name:     cname,
			Type:     ctype,
			Nullable: strings.ToUpper(nullable) == "YES",
		})
	}
	if err := rows.Err(); err != nil {
		return err
	}

	if format == "json" {
		return WriteOutput(os.Stdout, format, &SchemaResult{Columns: cols})
	}
	for _, c := range cols {
		nullStr := "NOT NULL"
		if c.Nullable {
			nullStr = "NULL"
		}
		fmt.Printf("%-25s %-15s %s\n", c.Name, c.Type, nullStr)
	}
	return nil
}

// openSchemaDB opens an in-memory DuckDB with the schema applied.
func openSchemaDB() (*db.Database, error) {
	database, err := db.Open("")
	if err != nil {
		return nil, fmt.Errorf("opening in-memory database: %w", err)
	}
	return database, nil
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
