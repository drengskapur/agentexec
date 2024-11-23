// File: pkg/combine/helpers.go
package combine

import (
	"bufio"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"go.uber.org/zap"
)

// shouldSkipFile determines if a file should be skipped based on ignore patterns, size, and binary content.
func shouldSkipFile(path string, info fs.FileInfo, gi IgnoreParser, maxFileSizeKB int, logger *zap.Logger, verbose bool) bool {
	relPath, _ := filepath.Rel(filepath.Dir(path), path)
	relPath = normalizePath(relPath)

	if gi.MatchesPath(relPath) {
		if verbose {
			logger.Debug("File matches ignore pattern", zap.String("file", path), zap.String("relPath", relPath))
		}
		return true
	}

	if isCommonBinaryExtension(path) {
		if verbose {
			logger.Debug("File has binary extension", zap.String("file", path), zap.String("extension", filepath.Ext(path)))
		}
		return true
	}

	if info.Size() > int64(maxFileSizeKB)*1024 {
		if verbose {
			logger.Debug("File exceeds size limit", zap.String("file", path), zap.Int64("sizeBytes", info.Size()), zap.Int("maxSizeKB", maxFileSizeKB))
		}
		return true
	}

	isBinary, err := isBinaryFile(path)
	if err != nil {
		logger.Error("Failed to check if file is binary", zap.String("file", path), zap.Error(err))
		return true
	}

	if isBinary {
		if verbose {
			logger.Debug("File is binary", zap.String("file", path))
		}
		return true
	}

	return false
}

// promptUser displays a message and waits for the user to enter 'y' or 'n'.
// Returns true if the user enters 'y' or 'yes' (case-insensitive), false otherwise.
func promptUser(message string) (bool, error) {
	fmt.Print(message)
	reader := bufio.NewReader(os.Stdin)
	response, err := reader.ReadString('\n')
	if err != nil {
		return false, err
	}
	response = strings.TrimSpace(strings.ToLower(response))
	return response == "y" || response == "yes", nil
}

// WriteCombinedFile writes the tree content and combined file contents to the output file.
func WriteCombinedFile(outputPath string, treeContent string, combinedContents []FileContent, logger *zap.Logger) error {
	logger.Debug("Writing combined content to output file", zap.String("combinedFile", outputPath))

	outFile, err := os.Create(outputPath)
	if err != nil {
		logger.Error("Failed to create output file", zap.String("file", outputPath), zap.Error(err))
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer func() {
		if err := outFile.Close(); err != nil {
			logger.Error("Failed to close output file", zap.String("file", outputPath), zap.Error(err))
		}
	}()

	writer := bufio.NewWriter(outFile)

	// Write tree content first
	if _, err := writer.WriteString(treeContent); err != nil {
		logger.Error("Failed to write tree content to combined file", zap.String("file", outputPath), zap.Error(err))
		return fmt.Errorf("failed to write tree content: %w", err)
	}

	// Write combined file contents
	for _, content := range combinedContents {
		if _, err := writer.WriteString(content.Content); err != nil {
			logger.Error("Failed to write content to combined file",
				zap.String("file", outputPath),
				zap.String("contentPath", content.Path),
				zap.Error(err))
			return fmt.Errorf("failed to write content: %w", err)
		}
	}

	if err := writer.Flush(); err != nil {
		logger.Error("Failed to flush output file", zap.String("file", outputPath), zap.Error(err))
		return fmt.Errorf("failed to flush output: %w", err)
	}

	return nil
}
