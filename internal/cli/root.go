package cli

import (
	"encoding/json"
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

// RootCmd returns the root cobra.Command for documentation generation.
func RootCmd() *cobra.Command {
	return rootCmd
}

func init() {
	rootCmd.AddCommand(newCompletionCmd())

	rootCmd.PersistentFlags().Bool("help-json", false, "Print help for all commands as machine-readable JSON")

	defaultRun := rootCmd.RunE
	rootCmd.RunE = func(cmd *cobra.Command, args []string) error {
		return handleHelpJSON(cmd, args, defaultRun)
	}

	rootCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		return handleHelpJSON(cmd, args, nil)
	}
}

func handleHelpJSON(cmd *cobra.Command, args []string, fallback func(*cobra.Command, []string) error) error {
	helpJSON, _ := cmd.Flags().GetBool("help-json")
	if !helpJSON {
		if fallback != nil {
			return fallback(cmd, args)
		}
		return nil
	}
	h := GenerateHelpJSON(rootCmd)
	data, err := json.MarshalIndent(h, "", "  ")
	if err != nil {
		return fmt.Errorf("generating help JSON: %w", err)
	}
	fmt.Println(string(data))
	os.Exit(0)
	return nil
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
