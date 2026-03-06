package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var infoCmd = &cobra.Command{
	Use:   "info <file.ifc>",
	Short: "Quick inspection of an IFC file without full import",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("info not yet implemented")
	},
}

func init() {
	rootCmd.AddCommand(infoCmd)
}
