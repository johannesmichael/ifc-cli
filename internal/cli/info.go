package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var infoCmd = &cobra.Command{
	Use:   "info <file.ifc>",
	Short: "Quick inspection of an IFC file without full import",
	Long: `Quickly inspect an IFC file's header metadata without performing a full import.

Displays the IFC schema version, originating system, preprocessor, file
description, and entity count summary. Use this to verify a file before
importing or to check which IFC version a model uses.`,
	Example: `  # Inspect file metadata
  ifc-to-db info model.ifc

  # Machine-readable output
  ifc-to-db info model.ifc --output-format json`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		outputFormat, _ := cmd.Flags().GetString("output-format")
		fmt.Printf("info: file=%s output-format=%s\n", args[0], outputFormat)
	},
}

func init() {
	infoCmd.Flags().String("output-format", "text", "Output format: text or json")

	infoCmd.RegisterFlagCompletionFunc("output-format", func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		return []string{"text", "json"}, cobra.ShellCompDirectiveNoFileComp
	})

	rootCmd.AddCommand(infoCmd)
}
