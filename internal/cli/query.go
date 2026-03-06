package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var queryCmd = &cobra.Command{
	Use:   "query <database.duckdb> <sql>",
	Short: "Run SQL against an imported DuckDB database",
	Args:  cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("query not yet implemented")
	},
}

func init() {
	rootCmd.AddCommand(queryCmd)
}
