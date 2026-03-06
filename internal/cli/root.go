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

Typical workflow:
  1. ifc-to-db info model.ifc                    # inspect the file
  2. ifc-to-db import model.ifc -o model.duckdb  # parse and import
  3. ifc-to-db schema --tables                   # see available tables
  4. ifc-to-db query model.duckdb "SELECT ..."   # run queries`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
