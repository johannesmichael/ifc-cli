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
		outputFormat, _ := cmd.Flags().GetString("output-format")
		fmt.Printf("info: file=%s output-format=%s\n", args[0], outputFormat)
	},
}

func init() {
	infoCmd.Flags().String("output-format", "text", "Output format: text or json")

	rootCmd.AddCommand(infoCmd)
}
