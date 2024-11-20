package combine

import (
	"bufio"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"omnivex/pkg/ignore"

	"go.uber.org/zap"
)

// Execute is the entry point for the combine package.
// It initializes the logger and starts the combination process.
func Execute(logger *zap.Logger) error {
	if logger == nil {
		var err error
		logger, err = zap.NewProduction()
		if err != nil {
			return fmt.Errorf("failed to initialize logger: %w", err)
		}
		defer logger.Sync()
	}

	args := &Arguments{
		Directory:     "./",               // Directory to process
		Output:        "debug/output.txt", // Output file path
		Tree:          "debug/tree.txt",   // Tree structure file path
		GlobalIgnore:  "",                 // Global ignore file path
		MaxFileSizeKB: 1024,               // Max file size in KB
		MaxWorkers:    4,                  // Number of worker threads
	}

	if err := RunCombine(args, logger); err != nil {
		logger.Error("Failed to execute combine process", zap.Error(err))
		return fmt.Errorf("combine execution failed: %w", err)
	}
	return nil
}

// RunCombine orchestrates the file combination process.
// It processes arguments, loads ignore patterns, collects files, and generates outputs.
func RunCombine(args *Arguments, logger *zap.Logger) error {
	startTime := time.Now()
	logger.Info("Starting combination process", zap.String("directory", args.Directory))

	// Resolve the parent directory's absolute path
	parentDir, err := filepath.Abs(args.Directory)
	if err != nil {
		logger.Error("Failed to resolve directory path", zap.Error(err))
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Load ignore patterns from local and global files
	gi, err := ignore.LoadIgnoreFiles(filepath.Join(parentDir, ".combineignore"), args.GlobalIgnore)
	if err != nil {
		logger.Error("Failed to load ignore patterns", zap.Error(err))
		return fmt.Errorf("failed to load ignore patterns: %w", err)
	}

	// Traverse the directory tree and collect files to process
	filesToProcess, err := traverseAndCollectFiles(parentDir, args.MaxFileSizeKB, gi, logger)
	if err != nil {
		logger.Error("Failed to collect files", zap.Error(err))
		return fmt.Errorf("failed to collect files: %w", err)
	}

	// Combine collected files into the output
	if err := processFiles(filesToProcess, parentDir, args.Output, args.MaxWorkers, logger); err != nil {
		logger.Error("Failed to process files", zap.Error(err))
		return fmt.Errorf("failed to process files: %w", err)
	}

	// Generate a directory tree structure
	if err := generateTreeStructure(parentDir, args.Tree, gi, logger); err != nil {
		logger.Error("Failed to generate tree structure", zap.Error(err))
		return fmt.Errorf("failed to generate tree structure: %w", err)
	}

	logger.Info("Combination process completed", zap.Duration("elapsed", time.Since(startTime)))
	return nil
}

// traverseAndCollectFiles walks through the directory tree and collects files to process.
// It skips files or directories based on the provided ignore patterns.
func traverseAndCollectFiles(parentDir string, maxFileSizeKB int, gi *ignore.GitIgnore, logger *zap.Logger) ([]string, error) {
	var files []string
	err := filepath.WalkDir(parentDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			logger.Warn("Error accessing path", zap.String("path", path), zap.Error(err))
			return nil
		}

		relPath, _ := filepath.Rel(parentDir, path)
		if d.IsDir() && gi.MatchesPath(relPath) {
			return filepath.SkipDir // Skip ignored directories
		}
		if !d.IsDir() && gi.MatchesPath(relPath) {
			return nil // Skip ignored files
		}
		if !d.IsDir() {
			info, _ := d.Info()
			if info.Size() <= int64(maxFileSizeKB)*1024 {
				files = append(files, path)
			}
		}
		return nil
	})
	return files, err
}

// processFiles reads and combines files into a single output file.
func processFiles(files []string, parentDir, output string, maxWorkers int, logger *zap.Logger) error {
	var wg sync.WaitGroup
	jobs := make(chan string, len(files))
	results := make(chan string, len(files))

	// Start worker goroutines
	for w := 0; w < maxWorkers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for file := range jobs {
				results <- processSingleFile(file, parentDir)
			}
		}()
	}

	// Send files to the job channel
	for _, file := range files {
		jobs <- file
	}
	close(jobs)

	// Close results channel after all workers finish
	go func() {
		wg.Wait()
		close(results)
	}()

	// Write results to the output file
	outFile, err := os.Create(output)
	if err != nil {
		logger.Error("Failed to create output file", zap.String("file", output), zap.Error(err))
		return err
	}
	defer outFile.Close()

	writer := bufio.NewWriter(outFile)
	for content := range results {
		_, _ = writer.WriteString(content)
	}
	return writer.Flush()
}

// processSingleFile reads and formats the content of a single file.
func processSingleFile(path, parentDir string) string {
	relPath, _ := filepath.Rel(parentDir, path)
	content := fmt.Sprintf("# File: %s\n\n", relPath)

	file, err := os.Open(path)
	if err != nil {
		return content + fmt.Sprintf("Error reading file: %v\n", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		content += scanner.Text() + "\n"
	}
	return content
}

// generateTreeStructure creates a directory tree structure and writes it to a file.
func generateTreeStructure(parentDir, outputFile string, gi *ignore.GitIgnore, logger *zap.Logger) error {
	var tree strings.Builder

	err := filepath.Walk(parentDir, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			logger.Warn("Error walking directory", zap.String("path", path), zap.Error(err))
			return nil
		}

		relPath, _ := filepath.Rel(parentDir, path)
		if gi.MatchesPath(relPath) {
			return nil // Skip ignored paths
		}

		indent := strings.Repeat("  ", strings.Count(relPath, string(os.PathSeparator)))
		tree.WriteString(fmt.Sprintf("%s%s\n", indent, info.Name()))
		return nil
	})
	if err != nil {
		logger.Error("Failed to walk directory for tree generation", zap.Error(err))
		return err
	}

	return os.WriteFile(outputFile, []byte(tree.String()), 0644)
}
