package cli

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
)

var BuildDate = "unknown"

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version, Go version, and build date",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("ifc-to-db %s\n", Version)
		fmt.Printf("go        %s\n", runtime.Version())
		fmt.Printf("built     %s\n", BuildDate)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
