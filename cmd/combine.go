// File: cmd/combine.go
package cmd

import (
	"fmt"

	"agentexec/pkg/combine"

	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

// combineCmd represents the combine command
var combineCmd = &cobra.Command{
	Use:   "combine [paths...]",
	Short: "Combine multiple files into a single output",
	Long: `Combine multiple files into a single output.

This command allows you to merge multiple files into a single output file.
You can specify various options such as the output path, tree structure output,
maximum file size, number of concurrent workers, ignore patterns, and verbosity.`,
	Args: cobra.ArbitraryArgs, // Allow any number of positional arguments
	RunE: runCombine,          // Use RunE for enhanced error handling
}

// runCombine is the main execution function for the combine command.
func runCombine(cmd *cobra.Command, args []string) error {
	// Retrieve the logger from the context
	logger, err := getLogger(cmd)
	if err != nil {
		return err
	}

	// Parse flags with error handling
	combineArgs, err := parseFlags(cmd, args, logger)
	if err != nil {
		return err
	}

	// Execute the combine process with the provided arguments
	if err := combine.ExecuteWithArgs(combineArgs, logger); err != nil {
		logger.Fatal("Combine execution failed", zap.Error(err))
	}

	return nil
}

// getLogger retrieves the Zap logger from the command's context.
func getLogger(cmd *cobra.Command) (*zap.Logger, error) {
	ctx := cmd.Context()
	logger, ok := ctx.Value(loggerKey).(*zap.Logger)
	if !ok || logger == nil {
		return nil, fmt.Errorf("logger not initialized")
	}
	return logger, nil
}

// parseFlags parses and validates the flags for the combine command.
func parseFlags(cmd *cobra.Command, args []string, logger *zap.Logger) (combine.Arguments, error) {
	output, err := cmd.Flags().GetString("output")
	if err != nil {
		logger.Error("Failed to parse 'output' flag", zap.Error(err))
		return combine.Arguments{}, fmt.Errorf("invalid 'output' flag: %w", err)
	}

	tree, err := cmd.Flags().GetString("tree")
	if err != nil {
		logger.Error("Failed to parse 'tree' flag", zap.Error(err))
		return combine.Arguments{}, fmt.Errorf("invalid 'tree' flag: %w", err)
	}

	maxSize, err := cmd.Flags().GetInt("max-size")
	if err != nil {
		logger.Error("Failed to parse 'max-size' flag", zap.Error(err))
		return combine.Arguments{}, fmt.Errorf("invalid 'max-size' flag: %w", err)
	}

	workers, err := cmd.Flags().GetInt("workers")
	if err != nil {
		logger.Error("Failed to parse 'workers' flag", zap.Error(err))
		return combine.Arguments{}, fmt.Errorf("invalid 'workers' flag: %w", err)
	}

	ignorePatterns, err := cmd.Flags().GetStringSlice("ignore")
	if err != nil {
		logger.Error("Failed to parse 'ignore' flag", zap.Error(err))
		return combine.Arguments{}, fmt.Errorf("invalid 'ignore' flag: %w", err)
	}

	verbose, err := cmd.Flags().GetBool("verbose")
	if err != nil {
		logger.Error("Failed to parse 'verbose' flag", zap.Error(err))
		return combine.Arguments{}, fmt.Errorf("invalid 'verbose' flag: %w", err)
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

	return combineArgs, nil
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

	// Optionally, mark flags as required or provide validation here
	// For example:
	// combineCmd.MarkFlagRequired("output")
}
