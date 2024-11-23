// File: pkg/combine/tree.go
package combine

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"go.uber.org/zap"
)

// GenerateFullTree generates a complete tree structure for all input paths.
// It returns the tree as a string and any error encountered during generation.
func GenerateFullTree(paths []string, gi IgnoreParser, logger *zap.Logger) (string, error) {
	// Option 1: Using var without initialization
	var treeBuilder strings.Builder

	// Option 2: Using short variable declaration with initialization
	// Uncomment the following line and comment out the above line if you prefer this approach.
	// treeBuilder := strings.Builder{}

	for _, path := range paths {
		absPath, err := filepath.Abs(path)
		if err != nil {
			logger.Warn("Failed to get absolute path for tree generation", zap.String("path", path), zap.Error(err))
			continue
		}

		info, err := os.Stat(absPath)
		if err != nil {
			logger.Warn("Cannot stat path for tree generation", zap.String("path", absPath), zap.Error(err))
			continue
		}

		if info.IsDir() {
			// Add the directory root
			treeBuilder.WriteString(fmt.Sprintf("%s/\n", absPath))

			// Generate subtree
			subtree, err := generateTreeRecursively(absPath, absPath, gi, "", logger)
			if err != nil {
				logger.Warn("Failed to generate subtree", zap.String("directory", absPath), zap.Error(err))
				continue
			}
			if subtree != "" {
				treeBuilder.WriteString(subtree)
				treeBuilder.WriteString("\n")
			}
		} else {
			relPath, relErr := filepath.Rel(filepath.Dir(absPath), absPath)
			if relErr != nil {
				relPath = absPath // Fallback to absolute path if relative path fails
			}
			relPath = normalizePath(relPath)
			treeBuilder.WriteString(relPath + "\n")
		}
	}

	return treeBuilder.String(), nil
}

// generateTreeRecursively builds the tree structure recursively.
// It returns the subtree as a string and any error encountered.
func generateTreeRecursively(directory, parentDir string, gi IgnoreParser, prefix string, logger *zap.Logger) (string, error) {
	var output []string

	entries, err := os.ReadDir(directory)
	if err != nil {
		logger.Warn("Failed to read directory for tree structure", zap.String("directory", directory), zap.Error(err))
		return "", fmt.Errorf("failed to read directory '%s': %w", directory, err)
	}

	// Sort entries: directories first, then files, alphabetically
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].IsDir() != entries[j].IsDir() {
			return entries[i].IsDir()
		}
		return strings.ToLower(entries[i].Name()) < strings.ToLower(entries[j].Name())
	})

	for i, entry := range entries {
		connector := "├── "
		extension := "│   "
		if i == len(entries)-1 {
			connector = "└── "
			extension = "    "
		}

		entryPath := filepath.Join(directory, entry.Name())
		relPath, _ := filepath.Rel(parentDir, entryPath)
		relPath = normalizePath(relPath)

		if entry.IsDir() {
			if gi.MatchesPath(relPath) {
				logger.Debug("Skipping ignored directory in tree", zap.String("directory", entryPath))
				continue // Skip ignored directories
			}
			// Append '/' to directory names
			line := fmt.Sprintf("%s%s%s/", prefix, connector, entry.Name())
			output = append(output, line)
			// Generate subtree with updated prefix
			subtree, err := generateTreeRecursively(entryPath, parentDir, gi, prefix+extension, logger)
			if err != nil {
				logger.Warn("Failed to generate subtree", zap.String("directory", entryPath), zap.Error(err))
				continue
			}
			if subtree != "" {
				output = append(output, subtree)
			}
		} else {
			if !gi.MatchesPath(relPath) {
				line := fmt.Sprintf("%s%s%s", prefix, connector, entry.Name())
				output = append(output, line)
			}
		}
	}

	return strings.Join(output, "\n"), nil
}
