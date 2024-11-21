// cmd/combine.go
package cmd

import (
	"os"

	"omnivex/pkg/combine"

	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

// combineCmd represents the combine command
var combineCmd = &cobra.Command{
	Use:   "combine [paths...]",
	Short: "Combine multiple files into a single output",
	Long:  `Combine multiple files into a single output, designed for workflows like ChatGPT input preparation.`,
	Args:  cobra.ArbitraryArgs, // Allow any number of positional arguments
	Run: func(cmd *cobra.Command, args []string) {
		// Retrieve the logger from the context
		logger, ok := cmd.Context().Value(loggerKey).(*zap.Logger)
		if !ok || logger == nil {
			// If logger is not available, log to stderr and exit
			os.Stderr.WriteString("Logger not initialized\n")
			os.Exit(1)
		}

		// Parse flags with error handling
		output, err := cmd.Flags().GetString("output")
		if err != nil {
			logger.Fatal("Failed to parse 'output' flag", zap.Error(err))
		}
		tree, err := cmd.Flags().GetString("tree")
		if err != nil {
			logger.Fatal("Failed to parse 'tree' flag", zap.Error(err))
		}
		maxSize, err := cmd.Flags().GetInt("max-size")
		if err != nil {
			logger.Fatal("Failed to parse 'max-size' flag", zap.Error(err))
		}
		workers, err := cmd.Flags().GetInt("workers")
		if err != nil {
			logger.Fatal("Failed to parse 'workers' flag", zap.Error(err))
		}
		ignorePatterns, err := cmd.Flags().GetStringSlice("ignore")
		if err != nil {
			logger.Fatal("Failed to parse 'ignore' flag", zap.Error(err))
		}
		verbose, err := cmd.Flags().GetBool("verbose")
		if err != nil {
			logger.Fatal("Failed to parse 'verbose' flag", zap.Error(err))
		}

		// If no paths are specified, default to current directory
		paths := args
		if len(paths) == 0 {
			paths = []string{"./"}
		}

		// Define the arguments based on flags and positional arguments
		combineArgs := combine.Arguments{
			Paths:          paths,
			Output:         output,
			Tree:           tree,
			MaxFileSizeKB:  maxSize,
			MaxWorkers:     workers,
			IgnorePatterns: ignorePatterns, // Use ignore patterns from flags
			Verbose:        verbose,        // Verbose logging flag
		}

		// Execute the combine process with the provided arguments
		if err := combine.ExecuteWithArgs(combineArgs, logger); err != nil {
			logger.Fatal("Combine execution failed", zap.Error(err))
		}
	},
}

func init() {
	// Define flags specific to the combine command
	combineCmd.Flags().StringP("output", "o", "debug/combined.txt", "Path to the combined output file")
	combineCmd.Flags().StringP("tree", "t", "debug/tree.txt", "Path to the tree structure output file")
	combineCmd.Flags().IntP("max-size", "m", 10240, "Maximum file size to process in KB (default: 10240KB)")
	combineCmd.Flags().IntP("workers", "w", 4, "Number of concurrent workers for processing files (default: 4)")
	combineCmd.Flags().StringSliceP("ignore", "i", []string{
		".git/",
		".combineignore",
		"debug/",
	}, "Ignore patterns (e.g., \"*.git\", \"build/\")")
	combineCmd.Flags().BoolP("verbose", "v", false, "Enable verbose logging of skipped files")
}
