package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var schemaCmd = &cobra.Command{
	Use:   "schema",
	Short: "Print the DuckDB schema DDL",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("schema not yet implemented")
	},
}

func init() {
	rootCmd.AddCommand(schemaCmd)
}
