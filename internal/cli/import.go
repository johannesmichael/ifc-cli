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
		output, _ := cmd.Flags().GetString("output")
		memory, _ := cmd.Flags().GetBool("memory")
		skipProperties, _ := cmd.Flags().GetBool("skip-properties")
		skipGeometry, _ := cmd.Flags().GetBool("skip-geometry")
		skipRelationships, _ := cmd.Flags().GetBool("skip-relationships")
		skipQuantities, _ := cmd.Flags().GetBool("skip-quantities")
		only, _ := cmd.Flags().GetStringSlice("only")
		appendMode, _ := cmd.Flags().GetBool("append")
		batchSize, _ := cmd.Flags().GetInt("batch-size")
		quiet, _ := cmd.Flags().GetBool("quiet")
		verbose, _ := cmd.Flags().GetBool("verbose")
		logFile, _ := cmd.Flags().GetString("log-file")
		outputFormat, _ := cmd.Flags().GetString("output-format")

		fmt.Printf("import: file=%s\n", args[0])
		fmt.Printf("  output=%s memory=%v\n", output, memory)
		fmt.Printf("  skip-properties=%v skip-geometry=%v skip-relationships=%v skip-quantities=%v\n",
			skipProperties, skipGeometry, skipRelationships, skipQuantities)
		fmt.Printf("  only=%v append=%v batch-size=%d\n", only, appendMode, batchSize)
		fmt.Printf("  quiet=%v verbose=%v log-file=%s output-format=%s\n", quiet, verbose, logFile, outputFormat)
	},
}

func init() {
	f := importCmd.Flags()
	f.StringP("output", "o", "", "Output DuckDB file path (default: <input_name>.duckdb)")
	f.Bool("memory", false, "Use in-memory DuckDB (no file output)")
	f.Bool("skip-properties", false, "Skip property set denormalization")
	f.Bool("skip-geometry", false, "Skip geometry serialization")
	f.Bool("skip-relationships", false, "Skip relationship extraction")
	f.Bool("skip-quantities", false, "Skip quantity extraction")
	f.StringSlice("only", nil, "Run only specified phases (properties, quantities, relationships, spatial, geometry)")
	f.Bool("append", false, "Append to existing database instead of overwriting")
	f.Int("batch-size", 10000, "Number of entities per batch insert")
	f.BoolP("quiet", "q", false, "Suppress progress output")
	f.BoolP("verbose", "v", false, "Detailed logging")
	f.String("log-file", "", "Write log output to file")
	f.String("output-format", "text", "Output format: text or json")

	rootCmd.AddCommand(importCmd)
}
