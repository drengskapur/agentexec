// File: pkg/combine/execute.go
package combine

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"go.uber.org/zap"
)

// executeProcess encapsulates the main logic for combining files.
func executeProcess(args Arguments, logger *zap.Logger) error {
	logger.Debug("Starting combine process", zap.Strings("paths", args.Paths))

	// Ensure output and tree directories exist
	if err := ensureDirectory(filepath.Dir(args.Output), logger); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}
	if err := ensureDirectory(filepath.Dir(args.Tree), logger); err != nil {
		return fmt.Errorf("failed to create tree output directory: %w", err)
	}

	// Load ignore patterns from `.combineignore` files (local and global)
	var globalIgnorePath string
	if args.GlobalIgnoreFile != "" {
		globalIgnorePath = args.GlobalIgnoreFile
	} else {
		globalIgnorePath = os.Getenv("COMBINEIGNORE_GLOBAL") // Optional environment variable for global ignore file
	}

	gi, err := LoadIgnoreFiles(globalIgnorePath, logger)
	if err != nil {
		logger.Error("Failed to load ignore patterns", zap.Error(err))
		return fmt.Errorf("failed to load ignore patterns: %w", err)
	}
	logger.Debug("Loaded ignore patterns", zap.Int("totalPatterns", len(gi.patterns)))

	// Add command-line ignore patterns to the ignore parser
	if len(args.IgnorePatterns) > 0 {
		gi.CompileIgnoreLines(args.IgnorePatterns...)
		logger.Debug("Added command-line ignore patterns", zap.Int("count", len(args.IgnorePatterns)))
	}

	// Collect files and binaries
	collected, err := CollectFiles(args.Paths, gi, args.MaxFileSizeKB, logger, args.Verbose)
	if err != nil {
		logger.Error("Failed to collect files", zap.Error(err))
		return fmt.Errorf("failed to collect files: %w", err)
	}

	// Warn about binary files
	if len(collected.Binary) > 0 {
		logger.Warn("Detected binary files. These files are not included in the combined output.",
			zap.Int("binaryFileCount", len(collected.Binary)),
			zap.Strings("binaryFiles", collected.Binary))

		shouldContinue, err := promptUser(fmt.Sprintf(
			"Detected %d binary files. Do you want to continue and exclude these files? (y/n): ", len(collected.Binary)))
		if err != nil {
			logger.Error("Failed to read user input", zap.Error(err))
			return fmt.Errorf("failed to read user input: %w", err)
		}

		if !shouldContinue {
			logger.Info("User chose to abort the combine process due to detected binary files.")
			return nil
		}
	}

	// Warn if no files remain after filtering
	if len(collected.Regular) == 0 {
		logger.Warn("No files to process after filtering.")
		return nil
	}

	// Process files concurrently
	combinedContents, err := ProcessFilesConcurrently(collected.Regular, args.MaxWorkers, filepath.Dir(args.Paths[0]), logger)
	if err != nil {
		logger.Error("Failed to process files", zap.Error(err))
		return fmt.Errorf("failed to process files: %w", err)
	}

	// Sort files for consistent output
	sort.Slice(combinedContents, func(i, j int) bool {
		return combinedContents[i].Path < combinedContents[j].Path
	})
	logger.Debug("Sorted processed files")

	// Generate tree structure
	treeContent, err := GenerateFullTree(args.Paths, gi, logger)
	if err != nil {
		logger.Error("Failed to generate tree structure", zap.Error(err))
		return fmt.Errorf("failed to generate tree structure: %w", err)
	}

	// Write tree structure to file
	if err := writeToFile(args.Tree, []byte(treeContent), 0644, logger); err != nil {
		return fmt.Errorf("failed to write tree structure: %w", err)
	}

	// Write combined contents to output file
	if err := WriteCombinedFile(args.Output, treeContent, combinedContents, logger); err != nil {
		logger.Error("Failed to write combined file", zap.String("combinedFile", args.Output), zap.Error(err))
		return fmt.Errorf("failed to write combined file: %w", err)
	}

	logger.Info("Successfully combined files",
		zap.String("outputFile", args.Output),
		zap.Int("totalFiles", len(combinedContents)),
	)
	return nil
}

// ensureDirectory ensures a directory exists, creating it if necessary.
func ensureDirectory(path string, logger *zap.Logger) error {
	if err := os.MkdirAll(path, os.ModePerm); err != nil {
		logger.Error("Failed to create directory", zap.String("path", path), zap.Error(err))
		return err
	}
	logger.Debug("Ensured directory exists", zap.String("path", path))
	return nil
}

// writeToFile writes data to a file and logs the operation.
func writeToFile(path string, data []byte, perm os.FileMode, logger *zap.Logger) error {
	if err := os.WriteFile(path, data, perm); err != nil {
		logger.Error("Failed to write file", zap.String("path", path), zap.Error(err))
		return err
	}
	logger.Debug("Successfully wrote file", zap.String("path", path))
	return nil
}
