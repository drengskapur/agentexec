package combine

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"go.uber.org/zap"
)

// ProcessSingleFile reads and formats the content of a single file.
func ProcessSingleFile(filePath, parentDir string, logger *zap.Logger) (FileContent, error) {
	logger.Debug("Processing file",
		zap.String("filePath", filePath),
		zap.String("parentDir", parentDir))

	// Ensure parentDir is an absolute path
	absParentDir, err := filepath.Abs(parentDir)
	if err != nil {
		logger.Warn("Failed to determine absolute path for parentDir",
			zap.String("parentDir", parentDir),
			zap.Error(err))
		absParentDir = parentDir // Fallback to original value
	}

	// Attempt to calculate the relative path
	relativePath, relErr := filepath.Rel(absParentDir, filePath)
	if relErr != nil {
		logger.Warn("Unable to determine relative path, using absolute path",
			zap.String("filePath", filePath),
			zap.String("parentDir", absParentDir),
			zap.Error(relErr))
		relativePath = filePath // Fallback to absolute path
	}
	relativePath = normalizePath(relativePath)

	// Construct the header for the file
	separatorLine := "# " + strings.Repeat("-", 78)
	header := fmt.Sprintf("\n\n%s\n# Source: %s #\n\n", separatorLine, relativePath)

	logger.Debug("Reading file content", zap.String("filePath", filePath))

	// Read file content
	fileBytes, readErr := os.ReadFile(filePath)
	if readErr != nil {
		logger.Error("Failed to read file",
			zap.String("filePath", filePath),
			zap.Error(readErr))
		return FileContent{}, fmt.Errorf("error reading file %s: %w", filePath, readErr)
	}

	logger.Debug("Successfully read file content",
		zap.String("filePath", filePath),
		zap.Int("contentSizeBytes", len(fileBytes)))

	// Return the processed file content
	return FileContent{
		Path:    relativePath,
		Content: header + string(fileBytes),
	}, nil
}
