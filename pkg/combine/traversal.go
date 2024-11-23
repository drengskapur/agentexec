// File: pkg/combine/traversal.go
package combine

import (
	"io/fs"
	"os"
	"path/filepath"

	"go.uber.org/zap"
)

// CollectFiles traverses the provided paths and collects regular and binary files.
func CollectFiles(paths []string, gi IgnoreParser, maxFileSizeKB int, logger *zap.Logger, verbose bool) (CollectedFiles, error) {
	var collected CollectedFiles
	logger.Debug("Starting file collection", zap.Int("pathCount", len(paths)))

	for _, path := range paths {
		absPath, err := filepath.Abs(path)
		if err != nil {
			logger.Warn("Failed to get absolute path", zap.String("path", path), zap.Error(err))
			continue
		}

		info, err := os.Stat(absPath)
		if err != nil {
			logger.Warn("Path does not exist or cannot be accessed", zap.String("path", absPath), zap.Error(err))
			continue
		}

		if info.IsDir() {
			logger.Debug("Processing directory", zap.String("dir", absPath))
			c, err := TraverseAndCollectFiles(absPath, gi, maxFileSizeKB, logger, verbose)
			if err != nil {
				logger.Warn("Failed to traverse directory", zap.String("dir", absPath), zap.Error(err))
				continue
			}
			collected.Regular = append(collected.Regular, c.Regular...)
			collected.Binary = append(collected.Binary, c.Binary...)
		} else {
			if shouldSkipFile(absPath, info, gi, maxFileSizeKB, logger, verbose) {
				continue
			}
			collected.Regular = append(collected.Regular, absPath)
		}
	}

	return collected, nil
}

// TraverseAndCollectFiles traverses a directory and collects files based on criteria.
func TraverseAndCollectFiles(parentDir string, gi IgnoreParser, maxFileSizeKB int, logger *zap.Logger, verbose bool) (CollectedFiles, error) {
	var collected CollectedFiles
	logger.Debug("Starting file traversal and collection", zap.String("parentDir", parentDir), zap.Int("maxFileSizeKB", maxFileSizeKB))

	err := filepath.WalkDir(parentDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			logger.Warn("Error accessing path during traversal", zap.String("path", path), zap.Error(err))
			return nil // Skip paths that cause errors
		}

		relPath, _ := filepath.Rel(parentDir, path)
		relPath = normalizePath(relPath)

		if d.IsDir() && gi.MatchesPath(relPath) {
			logger.Debug("Skipping ignored directory during traversal", zap.String("directory", path))
			return filepath.SkipDir
		}

		if !d.IsDir() && !gi.MatchesPath(relPath) {
			isBinary, err := isBinaryFile(path)
			if err != nil {
				logger.Warn("Failed to check if file is binary during traversal", zap.String("filePath", path), zap.Error(err))
				return nil
			}

			if isBinary {
				collected.Binary = append(collected.Binary, path)
				if verbose {
					logger.Debug("Detected binary file during traversal", zap.String("filePath", path))
				}
				return nil
			}

			info, err := d.Info()
			if err != nil {
				logger.Warn("Failed to get file info during traversal", zap.String("filePath", path), zap.Error(err))
				return nil
			}

			if info.Size() > int64(maxFileSizeKB)*1024 {
				if verbose {
					logger.Debug("Skipping file due to size limit during traversal", zap.String("filePath", path), zap.Int64("sizeBytes", info.Size()))
				}
				return nil
			}

			collected.Regular = append(collected.Regular, path)
			logger.Debug("Added file to processing list during traversal", zap.String("filePath", path))
		}

		return nil
	})

	if err != nil {
		logger.Error("Error during file traversal", zap.Error(err))
		return collected, err
	}

	logger.Debug("Completed file traversal and collection", zap.Int("regularFiles", len(collected.Regular)), zap.Int("binaryFiles", len(collected.Binary)))
	return collected, nil
}
