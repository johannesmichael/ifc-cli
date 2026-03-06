package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var Version = "0.1.0-dev"

var rootCmd = &cobra.Command{
	Use:   "ifc-to-db",
	Short: "Parse IFC files and write contents to DuckDB for SQL analysis",
	Long: `ifc-to-db parses IFC (Industry Foundation Classes) files and writes their
contents into a DuckDB database for downstream SQL analysis — property queries,
schedule integration, spatial analysis, and relationship traversal.

It supports IFC2X3, IFC4, and IFC4X3 STEP files, streaming large models with
constant memory usage and producing a queryable DuckDB database.

Typical workflow:
  1. ifc-to-db info model.ifc                    # inspect the file
  2. ifc-to-db import model.ifc -o model.duckdb  # parse and import
  3. ifc-to-db schema --tables                   # see available tables
  4. ifc-to-db query model.duckdb "SELECT ..."   # run queries

Exit codes:
  0  Success
  1  Parse error (malformed IFC data)
  2  File not found (input file does not exist)
  3  Database error (DuckDB open/write failure)
  4  Invalid arguments (bad flags or missing required args)
  5  Partial success (import completed with some entity errors)`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
