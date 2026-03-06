package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var importCmd = &cobra.Command{
	Use:   "import <file.ifc>",
	Short: "Parse an IFC file and write contents to DuckDB",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("import not yet implemented")
	},
}

func init() {
	rootCmd.AddCommand(importCmd)
}
