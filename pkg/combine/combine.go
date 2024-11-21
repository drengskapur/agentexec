package combine

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"omnivex/pkg/ignore"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"go.uber.org/zap"
)

// Execute is the main entry point for the combine package
func Execute(logger *zap.Logger) error {
	// Default configuration for the combine process
	args := Arguments{
		Directory:     "./",                 // Directory to process
		Output:        "debug/combined.txt", // Combined output file
		Tree:          "debug/tree.txt",     // Tree structure output file
		MaxFileSizeKB: 10240,                // 10MB max file size
		MaxWorkers:    4,                    // Number of workers
		GlobalIgnore:  ".combineignore",     // Default global ignore file
	}

	logger.Info("Starting combine process", zap.String("directory", args.Directory))

	// Ensure debug directory exists
	if err := os.MkdirAll(filepath.Dir(args.Output), os.ModePerm); err != nil {
		logger.Error("Failed to create output directory", zap.String("path", args.Output), zap.Error(err))
		return fmt.Errorf("failed to create output directory: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(args.Tree), os.ModePerm); err != nil {
		logger.Error("Failed to create tree output directory", zap.String("path", args.Tree), zap.Error(err))
		return fmt.Errorf("failed to create tree output directory: %w", err)
	}

	// Load ignore patterns
	gi, err := ignore.LoadIgnoreFiles(args.GlobalIgnore, "")
	if err != nil {
		logger.Error("Failed to load ignore patterns", zap.String("path", args.GlobalIgnore), zap.Error(err))
		return fmt.Errorf("failed to load ignore patterns: %w", err)
	}

	// Combine files and generate tree structure
	if err := CombineFiles(args, gi); err != nil {
		logger.Error("Combine process failed", zap.Error(err))
		return fmt.Errorf("combine process failed: %w", err)
	}

	logger.Info("Combine process completed successfully",
		zap.String("output", args.Output),
		zap.String("tree", args.Tree),
	)
	return nil
}

// ProcessSingleFile reads and formats the content of a single file
func ProcessSingleFile(filePath, parentDir string) string {
	separatorLine := "# " + strings.Repeat("-", 62) + " #"
	relativePath, _ := filepath.Rel(parentDir, filePath)
	content := fmt.Sprintf("\n\n%s\n# Source: %s #\n\n", separatorLine, relativePath)

	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Sprintf("%s# Error reading file: %v\n", content, err)
	}
	defer file.Close()

	reader := bufio.NewReader(file)
	var builder strings.Builder
	buf := make([]byte, 8192) // 8KB buffer
	for {
		n, err := reader.Read(buf)
		if n > 0 {
			builder.Write(buf[:n])
		}
		if err != nil {
			if !errors.Is(err, io.EOF) {
				return fmt.Sprintf("%s# Error reading file: %v\n", content, err)
			}
			break
		}
	}
	return content + builder.String()
}

// TraverseAndCollectFiles collects files to process based on the ignore rules and size limits
func TraverseAndCollectFiles(parentDir string, gi *ignore.GitIgnore, maxFileSizeKB int) ([]string, error) {
	var files []string
	err := filepath.WalkDir(parentDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // Ignore errors accessing files or directories
		}

		relPath, _ := filepath.Rel(parentDir, path)
		if d.IsDir() && gi.MatchesPath(relPath) {
			return filepath.SkipDir // Skip ignored directories
		}

		if !d.IsDir() && !gi.MatchesPath(relPath) {
			info, _ := d.Info()
			if info.Size() <= int64(maxFileSizeKB)*1024 { // Check file size
				files = append(files, path)
			}
		}
		return nil
	})
	return files, err
}

// Worker processes a single file and sends its content to the results channel
func Worker(jobs <-chan string, results chan<- FileContent, parentDir string, wg *sync.WaitGroup) {
	defer wg.Done()
	for file := range jobs {
		content := ProcessSingleFile(file, parentDir)
		results <- FileContent{Path: file, Content: content}
	}
}

// GenerateTreeStructure builds a visual tree representation of the directory
func GenerateTreeStructure(directory, parentDir string, gi *ignore.GitIgnore, prefix string) string {
	var output []string

	entries, _ := os.ReadDir(directory)
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].IsDir() && !entries[j].IsDir() {
			return true
		}
		if !entries[i].IsDir() && entries[j].IsDir() {
			return false
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

		if entry.IsDir() {
			if gi.MatchesPath(relPath) {
				continue
			}
			output = append(output, fmt.Sprintf("%s%s%s", prefix, connector, entry.Name()))
			subtree := GenerateTreeStructure(entryPath, parentDir, gi, prefix+extension)
			if subtree != "" {
				output = append(output, subtree)
			}
		} else {
			if !gi.MatchesPath(relPath) {
				output = append(output, fmt.Sprintf("%s%s%s", prefix, connector, entry.Name()))
			}
		}
	}

	return strings.Join(output, "\n")
}

// CombineFiles orchestrates the combination of files and tree generation
func CombineFiles(args Arguments, gi *ignore.GitIgnore) error {
	// Collect files to process
	filesToProcess, err := TraverseAndCollectFiles(args.Directory, gi, args.MaxFileSizeKB)
	if err != nil {
		return fmt.Errorf("failed to traverse directory: %w", err)
	}

	// Prepare channels for worker pool
	jobs := make(chan string, len(filesToProcess))
	results := make(chan FileContent, len(filesToProcess))

	var wg sync.WaitGroup
	for w := 0; w < args.MaxWorkers; w++ {
		wg.Add(1)
		go Worker(jobs, results, args.Directory, &wg)
	}

	// Send files to workers
	for _, file := range filesToProcess {
		jobs <- file
	}
	close(jobs)

	// Wait for all workers to finish and close results
	wg.Wait()
	close(results)

	// Collect and sort results
	var combinedContents []FileContent
	for result := range results {
		combinedContents = append(combinedContents, result)
	}
	sort.Slice(combinedContents, func(i, j int) bool {
		return combinedContents[i].Path < combinedContents[j].Path
	})

	// Write combined contents to output file
	outFile, err := os.Create(args.Output)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer outFile.Close()

	writer := bufio.NewWriter(outFile)
	for _, content := range combinedContents {
		_, err := writer.WriteString(content.Content)
		if err != nil {
			return fmt.Errorf("failed to write to output file: %w", err)
		}
	}
	if err := writer.Flush(); err != nil {
		return fmt.Errorf("failed to flush output writer: %w", err)
	}

	// Generate and write tree structure
	tree := GenerateTreeStructure(args.Directory, args.Directory, gi, "")
	if err := os.WriteFile(args.Tree, []byte(tree), 0644); err != nil {
		return fmt.Errorf("failed to write tree structure: %w", err)
	}

	return nil
}
