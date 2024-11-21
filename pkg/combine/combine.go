package combine

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"

	"go.uber.org/zap"
)

// ==============================
// Configuration and Data Structures
// ==============================

// Arguments holds the configuration for the combine process.
type Arguments struct {
	Paths          []string // Files/directories to process
	Output         string   // Combined output file
	Tree           string   // Tree structure output file
	MaxFileSizeKB  int      // Maximum file size in KB
	MaxWorkers     int      // Number of concurrent workers
	IgnorePatterns []string // Command-line specified ignore patterns
	Verbose        bool     // Enable verbose logging of skipped files
}

// FileContent represents the content of a single file.
type FileContent struct {
	Path    string // Relative path to the file
	Content string // Formatted content of the file
}

// ==============================
// Execute Function
// ==============================

// ExecuteWithArgs is the main entry point for the combine package with custom arguments.
func ExecuteWithArgs(args Arguments, logger *zap.Logger) error {
	logger.Info("Starting combine process", zap.Strings("paths", args.Paths))

	// Ensure output and tree directories exist
	if err := os.MkdirAll(filepath.Dir(args.Output), os.ModePerm); err != nil {
		logger.Error("Failed to create output directory", zap.String("path", args.Output), zap.Error(err))
		return fmt.Errorf("failed to create output directory: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(args.Tree), os.ModePerm); err != nil {
		logger.Error("Failed to create tree output directory", zap.String("path", args.Tree), zap.Error(err))
		return fmt.Errorf("failed to create tree output directory: %w", err)
	}

	// Load ignore patterns from default ignore file
	gi, err := LoadIgnoreFiles(".combineignore", "", logger)
	if err != nil {
		logger.Error("Failed to load default ignore patterns", zap.Error(err))
		return fmt.Errorf("failed to load default ignore patterns: %w", err)
	}

	// Compile command-line ignore patterns and add them to the ignore parser
	if len(args.IgnorePatterns) > 0 {
		gi.CompileIgnoreLines(args.IgnorePatterns...)
		logger.Info("Added command-line ignore patterns", zap.Int("count", len(args.IgnorePatterns)))
	}

	// Combine files and generate tree structure
	if err := CombineFiles(args, gi, logger); err != nil {
		logger.Error("Combine process failed", zap.Error(err))
		return fmt.Errorf("combine process failed: %w", err)
	}

	logger.Info("Combine process completed successfully",
		zap.String("output", args.Output),
		zap.String("tree", args.Tree),
	)
	return nil
}

// ==============================
// File Processing Functions
// ==============================

// CombineFiles orchestrates the combination of files and tree generation.
func CombineFiles(args Arguments, gi IgnoreParser, logger *zap.Logger) error {
	logger.Info("Starting file combination process",
		zap.Strings("inputPaths", args.Paths),
		zap.String("outputFile", args.Output),
		zap.Int("maxFileSizeKB", args.MaxFileSizeKB),
		zap.Int("maxWorkers", args.MaxWorkers))

	var allFilesToProcess []string

	// Collect files to process for each path
	for _, path := range args.Paths {
		absPath, err := filepath.Abs(path)
		if err != nil {
			logger.Warn("Failed to get absolute path",
				zap.String("path", path),
				zap.Error(err))
			continue
		}

		info, err := os.Stat(absPath)
		if err != nil {
			logger.Warn("Path does not exist or cannot be accessed",
				zap.String("path", absPath),
				zap.Error(err))
			continue
		}

		if info.IsDir() {
			parentDir := absPath
			logger.Debug("Processing directory",
				zap.String("dir", absPath),
				zap.String("parentDir", parentDir))

			files, err := TraverseAndCollectFiles(absPath, gi, args.MaxFileSizeKB, logger, args.Verbose)
			if err != nil {
				logger.Warn("Failed to traverse directory",
					zap.String("dir", absPath),
					zap.Error(err))
				continue
			}
			logger.Info("Collected files from directory",
				zap.String("dir", absPath),
				zap.Int("fileCount", len(files)))
			allFilesToProcess = append(allFilesToProcess, files...)
		} else if !shouldSkipFile(absPath, info, gi, args.MaxFileSizeKB, logger, args.Verbose) {
			logger.Debug("Adding single file to process",
				zap.String("file", absPath))
			allFilesToProcess = append(allFilesToProcess, absPath)
		}
	}

	if len(allFilesToProcess) == 0 {
		logger.Warn("No files to process")
		return nil
	}

	logger.Info("Starting file processing",
		zap.Int("totalFiles", len(allFilesToProcess)))

	// Process files concurrently
	jobs := make(chan string, len(allFilesToProcess))
	results := make(chan FileContent, len(allFilesToProcess))

	var wg sync.WaitGroup
	numWorkers := args.MaxWorkers
	if numWorkers <= 0 {
		numWorkers = runtime.NumCPU()
		logger.Info("Adjusted worker count",
			zap.Int("workers", numWorkers))
	}

	logger.Debug("Initializing worker pool",
		zap.Int("workers", numWorkers))

	for w := 0; w < numWorkers; w++ {
		wg.Add(1)
		workerLogger := logger.With(zap.Int("workerID", w))
		go Worker(w, jobs, results, filepath.Dir(args.Paths[0]), &wg, workerLogger)
	}

	// Send files to workers
	logger.Debug("Distributing files to workers")
	for _, file := range allFilesToProcess {
		jobs <- file
	}
	close(jobs)
	logger.Debug("All files distributed to workers")

	// Collect results in a separate goroutine
	var combinedContents []FileContent
	done := make(chan bool)
	go func() {
		for result := range results {
			logger.Debug("Received processed file",
				zap.String("file", result.Path))
			combinedContents = append(combinedContents, result)
		}
		done <- true
	}()

	// Wait for all workers to finish
	wg.Wait()
	close(results)
	<-done

	logger.Info("All files processed",
		zap.Int("processedFiles", len(combinedContents)))

	// Sort files for consistent output
	sort.Slice(combinedContents, func(i, j int) bool {
		return combinedContents[i].Path < combinedContents[j].Path
	})
	logger.Debug("Sorted processed files")

	// Create output file
	if err := os.MkdirAll(filepath.Dir(args.Output), 0755); err != nil {
		logger.Error("Failed to create output directory",
			zap.String("dir", filepath.Dir(args.Output)),
			zap.Error(err))
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	outFile, err := os.Create(args.Output)
	if err != nil {
		logger.Error("Failed to create output file",
			zap.String("file", args.Output),
			zap.Error(err))
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer func() {
		if err := outFile.Close(); err != nil {
			logger.Error("Failed to close output file",
				zap.String("file", args.Output),
				zap.Error(err))
		}
	}()

	logger.Debug("Writing combined content to output file",
		zap.String("file", args.Output))

	// Write combined contents
	writer := bufio.NewWriter(outFile)
	for _, content := range combinedContents {
		if _, err := writer.WriteString(content.Content); err != nil {
			logger.Error("Failed to write content to file",
				zap.String("file", args.Output),
				zap.String("contentPath", content.Path),
				zap.Error(err))
			return fmt.Errorf("failed to write content: %w", err)
		}
	}

	if err := writer.Flush(); err != nil {
		logger.Error("Failed to flush output file",
			zap.String("file", args.Output),
			zap.Error(err))
		return fmt.Errorf("failed to flush output: %w", err)
	}

	logger.Info("Successfully combined files",
		zap.String("outputFile", args.Output),
		zap.Int("totalFiles", len(combinedContents)))

	// Generate and write tree structure
	logger.Info("Writing tree structure to file", zap.String("tree", args.Tree))

	treeBuilder := strings.Builder{}
	for _, path := range args.Paths {
		info, err := os.Stat(path)
		if err != nil {
			logger.Warn("Cannot stat path for tree", zap.String("path", path), zap.Error(err))
			continue
		}

		if info.IsDir() {
			tree := GenerateTreeStructure(path, path, gi, "", logger)
			treeBuilder.WriteString(tree)
			treeBuilder.WriteString("\n")
		} else {
			relPath, relErr := filepath.Rel(filepath.Dir(path), path)
			if relErr != nil {
				relPath = path // Fallback to absolute path if relative path fails
			}
			relPath = normalizePath(relPath)
			treeBuilder.WriteString(relPath)
			treeBuilder.WriteString("\n")
		}
	}
	treeContent := treeBuilder.String()
	if err := os.WriteFile(args.Tree, []byte(treeContent), 0644); err != nil {
		logger.Error("Failed to write tree structure", zap.String("tree", args.Tree), zap.Error(err))
		return fmt.Errorf("failed to write tree structure: %w", err)
	}

	return nil
}

// ProcessSingleFile reads and formats the content of a single file.
func ProcessSingleFile(filePath, parentDir string, logger *zap.Logger) (FileContent, error) {
	logger.Debug("Processing file",
		zap.String("file", filePath),
		zap.String("parentDir", parentDir))

	separatorLine := "# " + strings.Repeat("-", 62) + " #"
	relativePath, err := filepath.Rel(parentDir, filePath)
	if parentDir == "" || err != nil {
		logger.Warn("Unable to determine relative path, using absolute",
			zap.String("file", filePath),
			zap.String("parentDir", parentDir),
			zap.Error(err))
		relativePath = filePath
	}
	relativePath = normalizePath(relativePath)

	header := fmt.Sprintf("\n\n%s\n# Source: %s #\n\n", separatorLine, relativePath)

	logger.Debug("Reading file content",
		zap.String("file", filePath))
	fileContent, err := os.ReadFile(filePath)
	if err != nil {
		logger.Error("Failed to read file",
			zap.String("file", filePath),
			zap.Error(err))
		return FileContent{}, fmt.Errorf("error reading file %s: %w", filePath, err)
	}

	logger.Debug("Successfully processed file",
		zap.String("file", filePath),
		zap.Int("contentBytes", len(fileContent)))

	return FileContent{
		Path:    relativePath,
		Content: header + string(fileContent),
	}, nil
}

// Worker processes files from the jobs channel and sends results to the results channel.
func Worker(id int, jobs <-chan string, results chan<- FileContent, parentDir string, wg *sync.WaitGroup, logger *zap.Logger) {
	defer wg.Done()
	logger.Debug("Worker started", zap.Int("workerID", id))

	for file := range jobs {
		logger.Debug("Worker processing file",
			zap.Int("workerID", id),
			zap.String("file", file))

		if content, err := ProcessSingleFile(file, parentDir, logger); err == nil {
			results <- content
			logger.Debug("Worker completed file processing",
				zap.Int("workerID", id),
				zap.String("file", file))
		} else {
			logger.Error("Worker failed to process file",
				zap.Int("workerID", id),
				zap.String("file", file),
				zap.Error(err))
		}
	}

	logger.Debug("Worker finished", zap.Int("workerID", id))
}

// ==============================
// File Traversal and Collection
// ==============================

// TraverseAndCollectFiles collects files to process based on the ignore rules, size limits, and binary detection
func TraverseAndCollectFiles(parentDir string, gi IgnoreParser, maxFileSizeKB int, logger *zap.Logger, verbose bool) ([]string, error) {
	var files []string
	err := filepath.WalkDir(parentDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			logger.Warn("Error accessing path", zap.String("path", path), zap.Error(err))
			return nil // Ignore errors accessing files or directories
		}

		relPath, _ := filepath.Rel(parentDir, path)
		relPath = normalizePath(relPath)

		if d.IsDir() && gi.MatchesPath(relPath) {
			return filepath.SkipDir // Skip ignored directories
		}

		if !d.IsDir() && !gi.MatchesPath(relPath) {
			// Quick check for common binary extensions first
			if isCommonBinaryExtension(path) {
				if verbose {
					logger.Debug("Skipping common binary file", zap.String("file", path))
				}
				return nil
			}

			info, err := d.Info()
			if err != nil {
				logger.Warn("Failed to get file info", zap.String("file", path), zap.Error(err))
				return nil
			}

			// Check file size first
			if info.Size() > int64(maxFileSizeKB)*1024 {
				if verbose {
					logger.Debug("Skipping file due to size limit",
						zap.String("file", path),
						zap.Int64("sizeBytes", info.Size()))
				}
				return nil
			}

			// Only check for binary content if it passes other checks
			isBinary, err := isBinaryFile(path)
			if err != nil {
				logger.Warn("Failed to check if file is binary",
					zap.String("file", path),
					zap.Error(err))
				return nil
			}

			if isBinary {
				if verbose {
					logger.Debug("Skipping binary file", zap.String("file", path))
				}
				return nil
			}

			files = append(files, path)
		}
		return nil
	})
	return files, err
}

// ==============================
// Helper Functions
// ==============================

// shouldSkipFile determines if a file should be skipped based on ignore patterns, size, and binary content.
func shouldSkipFile(path string, info fs.FileInfo, gi IgnoreParser, maxFileSizeKB int, logger *zap.Logger, verbose bool) bool {
	relPath, _ := filepath.Rel(filepath.Dir(path), path)
	relPath = normalizePath(relPath)

	if gi.MatchesPath(relPath) {
		if verbose {
			logger.Debug("File matches ignore pattern",
				zap.String("file", path),
				zap.String("relPath", relPath))
		}
		return true
	}

	if isCommonBinaryExtension(path) {
		if verbose {
			logger.Debug("File has binary extension",
				zap.String("file", path),
				zap.String("extension", filepath.Ext(path)))
		}
		return true
	}

	if info.Size() > int64(maxFileSizeKB)*1024 {
		if verbose {
			logger.Debug("File exceeds size limit",
				zap.String("file", path),
				zap.Int64("size", info.Size()),
				zap.Int("maxSizeKB", maxFileSizeKB))
		}
		return true
	}

	isBinary, err := isBinaryFile(path)
	if err != nil {
		logger.Error("Failed to check if file is binary",
			zap.String("file", path),
			zap.Error(err))
		return true
	}

	if isBinary {
		if verbose {
			logger.Debug("File is binary",
				zap.String("file", path))
		}
		return true
	}

	return false
}

// normalizePath converts the OS-specific path separators to forward slashes.
func normalizePath(path string) string {
	return filepath.ToSlash(path)
}

// ==============================
// Binary Detection Functionality
// ==============================

// isBinaryFile checks if a file is likely to be binary by reading its first few bytes
// and checking for null bytes or a high ratio of non-printable characters
func isBinaryFile(filePath string) (bool, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return false, err
	}
	defer file.Close()

	// Read first 512 bytes to check content type
	buffer := make([]byte, 512)
	n, err := file.Read(buffer)
	if err != nil && !errors.Is(err, io.EOF) {
		return false, err
	}
	buffer = buffer[:n]

	// Check for null bytes (common in binary files)
	if bytes.Contains(buffer, []byte{0}) {
		return true, nil
	}

	// Count non-printable characters
	nonPrintable := 0
	for _, b := range buffer {
		if !isPrintable(b) {
			nonPrintable++
		}
	}

	// If more than 30% non-printable characters, consider it binary
	if len(buffer) == 0 {
		return false, nil // Empty files are considered text
	}
	return float64(nonPrintable)/float64(len(buffer)) > 0.3, nil
}

// isPrintable checks if a byte represents a printable ASCII character
func isPrintable(b byte) bool {
	return (b >= 32 && b <= 126) || b == '\n' || b == '\r' || b == '\t'
}

// Common binary file extensions to auto-ignore
var binaryExtensions = map[string]bool{
	".exe":      true,
	".dll":      true,
	".so":       true,
	".dylib":    true,
	".bin":      true,
	".obj":      true,
	".o":        true,
	".a":        true,
	".lib":      true,
	".pyc":      true,
	".pyo":      true,
	".class":    true,
	".jar":      true,
	".war":      true,
	".ear":      true,
	".png":      true,
	".jpg":      true,
	".jpeg":     true,
	".gif":      true,
	".bmp":      true,
	".ico":      true,
	".pdf":      true,
	".zip":      true,
	".tar":      true,
	".gz":       true,
	".7z":       true,
	".rar":      true,
	".db":       true,
	".sqlite":   true,
	".mp3":      true,
	".mp4":      true,
	".avi":      true,
	".mov":      true,
	".wmv":      true,
	".flac":     true,
	".m4a":      true,
	".mkv":      true,
	".wav":      true,
	".iso":      true,
	".dmg":      true,
	".pkg":      true,
	".deb":      true,
	".rpm":      true,
	".msi":      true,
	".apk":      true,
	".ipa":      true,
	".svg":      true,
	".webp":     true,
	".heic":     true,
	".psd":      true,
	".ttf":      true,
	".otf":      true,
	".woff":     true,
	".woff2":    true,
	".eot":      true,
	".dbf":      true,
	".mdb":      true,
	".accdb":    true,
	".bak":      true,
	".tmp":      true,
	".log":      true,
	".cache":    true,
	".swp":      true,
	".swo":      true,
	".DS_Store": true,
}

// isCommonBinaryExtension checks if the file has a known binary extension
func isCommonBinaryExtension(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return binaryExtensions[ext]
}

// ==============================
// Ignore Functionality
// ==============================

// IgnoreParser defines the interface for matching paths against ignore patterns.
type IgnoreParser interface {
	// MatchesPath returns true if the given path matches any of the ignore patterns.
	MatchesPath(path string) bool
	// MatchesPathWithPattern returns true and the matching IgnorePattern if the given path matches any ignore pattern.
	MatchesPathWithPattern(path string) (bool, *IgnorePattern)
}

// IgnorePattern encapsulates a compiled regular expression pattern,
// a negation flag, and metadata about the pattern's origin.
type IgnorePattern struct {
	Pattern *regexp.Regexp // Compiled regular expression for the pattern.
	Negate  bool           // Indicates if the pattern is a negation (starts with '!').
	LineNo  int            // Line number in the source (1-based).
	Line    string         // Original pattern line.
}

// GitIgnore represents a collection of ignore patterns.
type GitIgnore struct {
	patterns []*IgnorePattern // Slice of compiled ignore patterns.
	logger   *zap.Logger      // Logger for debug information.
}

// NewGitIgnore initializes a GitIgnore instance with a provided logger.
func NewGitIgnore(logger *zap.Logger) *GitIgnore {
	if logger == nil {
		// Fallback to a no-op logger if none is provided to avoid nil pointer dereferences
		logger = zap.NewNop()
	}
	return &GitIgnore{
		patterns: []*IgnorePattern{},
		logger:   logger,
	}
}

// LoadIgnoreFiles loads ignore patterns from local and global ignore files.
func LoadIgnoreFiles(localPath, globalPath string, logger *zap.Logger) (*GitIgnore, error) {
	gi := NewGitIgnore(logger) // Use the provided logger.

	// Initialize the .combineignore file with default patterns if it doesn't exist
	if localPath == "" {
		localPath = "./.combineignore"
		if _, err := os.Stat(localPath); os.IsNotExist(err) {
			// Create .combineignore with default ignore patterns if it does not exist
			defaultPatterns := []string{
				".git/",          // Ignore the .git directory
				".combineignore", // Ignore the .combineignore file itself
				"debug/",         // Ignore the debug directory
			}
			if err := os.WriteFile(localPath, []byte(strings.Join(defaultPatterns, "\n")), 0644); err != nil {
				gi.logger.Error("Failed to create .combineignore file", zap.String("file", localPath), zap.Error(err))
				return nil, fmt.Errorf("failed to create .combineignore file: %w", err)
			}
			gi.logger.Info("Created default .combineignore file", zap.String("file", localPath))
		}
	}

	// Load global ignore file if specified
	if globalPath != "" {
		if err := gi.CompileIgnoreFile(globalPath); err != nil && !os.IsNotExist(err) {
			gi.logger.Error("Failed to compile global ignore file", zap.String("file", globalPath), zap.Error(err))
			return nil, err
		}
	}

	// Load local ignore file if specified
	if localPath != "" {
		if err := gi.CompileIgnoreFile(localPath); err != nil && !os.IsNotExist(err) {
			gi.logger.Error("Failed to compile local ignore file", zap.String("file", localPath), zap.Error(err))
			return nil, err
		}
	}

	return gi, nil
}

// CompileIgnoreLines compiles a set of ignore pattern lines into a GitIgnore instance.
// It accepts a variadic number of pattern strings.
func (gi *GitIgnore) CompileIgnoreLines(lines ...string) {
	for i, line := range lines {
		pattern, negate := parsePatternLine(line, len(gi.patterns)+i+1, gi.logger)
		if pattern != nil {
			ip := &IgnorePattern{
				Pattern: pattern,
				Negate:  negate,
				LineNo:  len(gi.patterns) + i + 1, // 1-based line numbering.
				Line:    line,
			}
			gi.patterns = append(gi.patterns, ip)
			gi.logger.Debug("Compiled ignore pattern", zap.Int("lineNo", ip.LineNo), zap.String("pattern", ip.Line), zap.Bool("negate", ip.Negate))
		}
	}
}

// CompileIgnoreFile reads an ignore file from the given path, parses its lines,
// and compiles them into the GitIgnore instance.
func (gi *GitIgnore) CompileIgnoreFile(filePath string) error {
	content, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			gi.logger.Warn("Ignore file does not exist", zap.String("filePath", filePath))
			return nil // It's acceptable for the ignore file to not exist
		}
		gi.logger.Error("Failed to read ignore file", zap.String("filePath", filePath), zap.Error(err))
		return err
	}

	lines := strings.Split(string(content), "\n")
	for i, line := range lines {
		pattern, negate := parsePatternLine(line, i+1, gi.logger)
		if pattern != nil {
			ip := &IgnorePattern{
				Pattern: pattern,
				Negate:  negate,
				LineNo:  i + 1, // 1-based line numbering.
				Line:    line,
			}
			gi.patterns = append(gi.patterns, ip)
			gi.logger.Debug("Compiled ignore pattern from file", zap.String("filePath", filePath), zap.Int("lineNo", ip.LineNo), zap.String("pattern", ip.Line), zap.Bool("negate", ip.Negate))
		}
	}
	gi.logger.Info("Compiled ignore patterns from file", zap.String("filePath", filePath), zap.Int("patternCount", len(lines)))
	return nil
}

// MatchesPath checks if the given path matches any of the ignore patterns.
// It returns true if the path should be ignored.
func (gi *GitIgnore) MatchesPath(path string) bool {
	matches, _ := gi.MatchesPathWithPattern(path)
	return matches
}

// MatchesPathWithPattern checks if the given path matches any ignore pattern.
// It returns a boolean indicating a match and the specific IgnorePattern that matched.
func (gi *GitIgnore) MatchesPathWithPattern(path string) (bool, *IgnorePattern) {
	normalizedPath := normalizePath(path)

	matched := false
	var matchedPattern *IgnorePattern

	for _, pattern := range gi.patterns {
		if pattern.Pattern.MatchString(normalizedPath) {
			if pattern.Negate {
				matched = false
				matchedPattern = pattern
			} else {
				matched = true
				matchedPattern = pattern
			}
			// Note: The last matching pattern determines the outcome.
		}
	}

	return matched, matchedPattern
}

// Patterns returns the original pattern lines used to compile the GitIgnore.
func (gi *GitIgnore) Patterns() []string {
	var patterns []string
	for _, p := range gi.patterns {
		patterns = append(patterns, p.Line)
	}
	return patterns
}

// CompileIgnoreFileAndLines reads an ignore file and appends additional lines,
// compiling all into the existing GitIgnore instance.
func CompileIgnoreFileAndLines(filePath string, gi *GitIgnore, additionalLines ...string) error {
	// Compile patterns from the ignore file
	if err := gi.CompileIgnoreFile(filePath); err != nil {
		return err
	}

	// Compile additional lines
	gi.CompileIgnoreLines(additionalLines...)
	return nil
}

// parsePatternLine processes a single line from an ignore file and returns
// a compiled regular expression and a negation flag.
// Returns nil if the line is a comment or empty.
func parsePatternLine(line string, lineNo int, logger *zap.Logger) (*regexp.Regexp, bool) {
	trimmedLine := strings.TrimRight(line, "\r\n")

	// 1. Ignore empty lines
	if trimmedLine == "" {
		return nil, false
	}

	// 2. Ignore comments
	if strings.HasPrefix(trimmedLine, "#") {
		return nil, false
	}

	// 3. Trim surrounding whitespace
	trimmedLine = strings.TrimSpace(trimmedLine)

	// 4. Handle negation
	negate := false
	if strings.HasPrefix(trimmedLine, "!") {
		negate = true
		trimmedLine = strings.TrimPrefix(trimmedLine, "!")
	}

	// 5. Handle escaped characters '#' or '!'
	if strings.HasPrefix(trimmedLine, "\\#") || strings.HasPrefix(trimmedLine, "\\!") {
		trimmedLine = trimmedLine[1:]
	}

	// 6. Prepend '/' if pattern contains a wildcard in a directory and doesn't start with '/'
	if wildcardDirPattern.MatchString(trimmedLine) && !strings.HasPrefix(trimmedLine, "/") {
		trimmedLine = "/" + trimmedLine
	}

	// 7. Escape '.' characters
	escapedLine := escapeSpecialChars(trimmedLine)

	// 8. Replace '/**/' with "(/|/.+/)"
	escapedLine = handleDoubleStarPatterns(escapedLine)

	// 9. Convert wildcards '*' and '?' to regex equivalents
	regexPattern := wildcardToRegex(escapedLine)

	// 10. Anchor the pattern to match the entire path
	regexPattern = anchorPattern(regexPattern, trimmedLine)

	compiledRegex, err := regexp.Compile(regexPattern)
	if err != nil {
		// Log invalid regex patterns with line number
		logger.Error("Invalid regex pattern",
			zap.String("originalPattern", trimmedLine),
			zap.String("compiledRegex", regexPattern),
			zap.Int("lineNo", lineNo),
			zap.Error(err),
		)
		return nil, false
	}

	return compiledRegex, negate
}

// escapeSpecialChars escapes regex special characters except for '*', '?', and '/'.
func escapeSpecialChars(pattern string) string {
	var specialChars = `.+()|^$[]{}`
	for _, char := range specialChars {
		pattern = strings.ReplaceAll(pattern, string(char), `\`+string(char))
	}
	return pattern
}

// handleDoubleStarPatterns replaces '**' patterns with appropriate regex.
func handleDoubleStarPatterns(pattern string) string {
	// Replace "/**/" with "(/|/.+/)"
	pattern = doubleStarPattern1.ReplaceAllString(pattern, `(/|/.+/)`)

	// Replace "/**" with "(/.*)?"
	pattern = doubleStarPattern2.ReplaceAllString(pattern, `(/.*)?`)

	// Replace "**/" with "(.*/)?"
	pattern = doubleStarPattern3.ReplaceAllString(pattern, `(.*/)?`)

	return pattern
}

// wildcardToRegex converts wildcard patterns '*' and '?' to regex equivalents.
func wildcardToRegex(pattern string) string {
	// Replace '*' with '[^/]*' to match any character except '/'
	pattern = wildcardReplaceStar.ReplaceAllString(pattern, `[^/]*`)

	// Replace '?' with '.' to match any single character
	pattern = strings.ReplaceAll(pattern, "?", ".")
	return pattern
}

// anchorPattern anchors the regex pattern to match the entire path.
func anchorPattern(pattern string, originalPattern string) string {
	if strings.HasSuffix(originalPattern, "/") {
		pattern += "(|.*)$"
	} else {
		pattern += "(|/.*)$"
	}

	if strings.HasPrefix(pattern, "/") {
		return "^" + pattern
	}
	return "^(|.*/)" + pattern
}

// ==============================
// Precompiled Regular Expressions
// ==============================

var (
	// wildcardDirPattern detects patterns with wildcards in directories, e.g., "folder/*.ext"
	wildcardDirPattern = regexp.MustCompile(`[^/]\*/`)

	// doubleStarPattern1 matches "/**/" for replacement
	doubleStarPattern1 = regexp.MustCompile(`/\*\*/`)

	// doubleStarPattern2 matches "/**" at the end for replacement
	doubleStarPattern2 = regexp.MustCompile(`/\*\*$`)

	// doubleStarPattern3 matches "**/" at the beginning for replacement
	doubleStarPattern3 = regexp.MustCompile(`^\*\*/`)

	// wildcardReplaceStar replaces '*' with '[^/]*'
	wildcardReplaceStar = regexp.MustCompile(`\*`)
)

// ==============================
// Tree Structure Generation
// ==============================

// GenerateTreeStructure builds a visual tree representation of the directory.
func GenerateTreeStructure(directory, parentDir string, gi IgnoreParser, prefix string, logger *zap.Logger) string {
	var output []string

	entries, err := os.ReadDir(directory)
	if err != nil {
		logger.Warn("Failed to read directory for tree structure", zap.String("directory", directory), zap.Error(err))
		return "" // If directory can't be read, skip
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
			output = append(output, fmt.Sprintf("%s%s%s", prefix, connector, entry.Name()))
			subtree := GenerateTreeStructure(entryPath, parentDir, gi, prefix+extension, logger)
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
