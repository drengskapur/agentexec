// File: pkg/combine/ignore.go
package combine

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"go.uber.org/zap"
)

// IgnoreParser defines the interface for matching paths against ignore patterns.
type IgnoreParser interface {
	MatchesPath(path string) bool
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

// CombineIgnore represents a collection of ignore patterns.
type CombineIgnore struct {
	patterns []*IgnorePattern // Slice of compiled ignore patterns.
	logger   *zap.Logger      // Logger for debug information.
}

// NewCombineIgnore initializes a CombineIgnore instance with a provided logger.
func NewCombineIgnore(logger *zap.Logger) *CombineIgnore {
	if logger == nil {
		logger = zap.NewNop() // Use no-op logger if none is provided
	}
	return &CombineIgnore{
		patterns: []*IgnorePattern{},
		logger:   logger,
	}
}

// LoadIgnoreFiles loads ignore patterns from `.combineignore` files
// in the current directory and all parent directories, merging them hierarchically.
func LoadIgnoreFiles(globalPath string, logger *zap.Logger) (*CombineIgnore, error) {
	gi := NewCombineIgnore(logger)

	// Load global ignore file if specified
	if globalPath != "" {
		absGlobalPath, err := filepath.Abs(globalPath)
		if err == nil {
			if err := gi.CompileIgnoreFile(absGlobalPath); err != nil {
				logger.Warn("Failed to load global ignore file", zap.String("file", absGlobalPath), zap.Error(err))
			} else {
				logger.Debug("Loaded global ignore file", zap.String("file", absGlobalPath))
			}
		}
	}

	// Traverse directories to load `.combineignore` files from root to current directory
	startDir, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get current directory: %w", err)
	}

	var ignoreFiles []string
	currentDir := startDir
	loadedFiles := false // Track if any `.combineignore` file was loaded

	for {
		ignoreFilePath := filepath.Join(currentDir, ".combineignore")
		if _, err := os.Stat(ignoreFilePath); err == nil {
			ignoreFiles = append([]string{ignoreFilePath}, ignoreFiles...) // Prepend to ensure root patterns are loaded first
			loadedFiles = true
		}

		parentDir := filepath.Dir(currentDir)
		if parentDir == currentDir {
			break // Reached the root directory
		}
		currentDir = parentDir
	}

	// Compile patterns from all `.combineignore` files
	for _, file := range ignoreFiles {
		if err := gi.CompileIgnoreFile(file); err != nil {
			logger.Warn("Failed to compile .combineignore file", zap.String("file", file), zap.Error(err))
		} else {
			logger.Debug("Loaded .combineignore file", zap.String("file", file))
			fmt.Printf("Loaded ignore file: %s\n", file) // Print loaded file
		}
	}

	if !loadedFiles {
		fmt.Println("No .combineignore files were loaded.")
	} else {
		fmt.Println("One or more .combineignore files were successfully loaded.")
	}

	logger.Debug("Finished loading ignore files", zap.Int("totalPatterns", len(gi.patterns)))
	return gi, nil
}

// CompileIgnoreLines compiles a set of ignore pattern lines into the CombineIgnore instance.
func (gi *CombineIgnore) CompileIgnoreLines(lines ...string) {
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
			gi.logger.Debug("Compiled ignore pattern",
				zap.Int("lineNo", ip.LineNo),
				zap.String("pattern", ip.Line),
				zap.Bool("negate", ip.Negate))
		}
	}
}

// CompileIgnoreFile reads an ignore file, parses its lines, and compiles them into the CombineIgnore instance.
func (gi *CombineIgnore) CompileIgnoreFile(filePath string) error {
	gi.logger.Debug("Starting to compile ignore file", zap.String("filePath", filePath))
	content, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			gi.logger.Debug("Ignore file does not exist and will be skipped", zap.String("filePath", filePath))
			return nil
		}
		gi.logger.Error("Failed to read ignore file", zap.String("filePath", filePath), zap.Error(err))
		return err
	}

	lines := strings.Split(string(content), "\n")
	gi.logger.Debug("Read ignore file lines", zap.String("filePath", filePath), zap.Int("lineCount", len(lines)))
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
			gi.logger.Debug("Compiled ignore pattern from file",
				zap.String("filePath", filePath),
				zap.Int("lineNo", ip.LineNo),
				zap.String("pattern", ip.Line),
				zap.Bool("negate", ip.Negate))
		} else {
			gi.logger.Debug("Skipped empty or comment line in ignore file",
				zap.String("filePath", filePath),
				zap.Int("lineNo", i+1))
		}
	}
	gi.logger.Debug("Compiled ignore patterns from file", zap.String("filePath", filePath), zap.Int("patternCount", len(lines)))
	return nil
}

// MatchesPath checks if the given path matches any of the ignore patterns.
func (gi *CombineIgnore) MatchesPath(path string) bool {
	matches, _ := gi.MatchesPathWithPattern(path)
	return matches
}

// MatchesPathWithPattern checks if the given path matches any ignore pattern.
// It returns a boolean indicating a match and the specific IgnorePattern that matched.
func (gi *CombineIgnore) MatchesPathWithPattern(path string) (bool, *IgnorePattern) {
	normalizedPath := normalizePath(path)
	gi.logger.Debug("Normalized path for matching", zap.String("path", normalizedPath))

	matched := false
	var matchedPattern *IgnorePattern

	for _, pattern := range gi.patterns {
		if pattern.Pattern.MatchString(normalizedPath) {
			gi.logger.Debug("Path matches pattern",
				zap.String("path", normalizedPath),
				zap.String("pattern", pattern.Line),
				zap.Bool("negate", pattern.Negate),
			)

			if pattern.Negate {
				matched = false
				matchedPattern = pattern
			} else {
				matched = true
				matchedPattern = pattern
			}
		}
	}

	if !matched {
		gi.logger.Debug("No pattern matched path", zap.String("path", normalizedPath))
	}

	return matched, matchedPattern
}

// parsePatternLine processes a single line from an ignore file and returns
// a compiled regular expression and a negation flag.
// Returns nil if the line is a comment or empty.
func parsePatternLine(line string, lineNo int, logger *zap.Logger) (*regexp.Regexp, bool) {
	trimmedLine := strings.TrimSpace(line)

	// Ignore empty lines and comments
	if trimmedLine == "" || strings.HasPrefix(trimmedLine, "#") {
		return nil, false
	}

	// Handle negation
	negate := false
	if strings.HasPrefix(trimmedLine, "!") {
		negate = true
		trimmedLine = strings.TrimPrefix(trimmedLine, "!")
	}

	// Escape special characters in the pattern
	escapedLine := escapeSpecialChars(trimmedLine)

	// Replace '**' patterns with appropriate regex
	escapedLine = handleDoubleStarPatterns(escapedLine)

	// Convert wildcards '*' and '?' to regex equivalents
	regexPattern := wildcardToRegex(escapedLine)

	// Anchor the pattern to match the entire path
	regexPattern = anchorPattern(regexPattern, trimmedLine)

	// Compile the regex
	compiledRegex, err := regexp.Compile("^" + regexPattern)
	if err != nil {
		logger.Error("Invalid regex pattern",
			zap.String("pattern", trimmedLine),
			zap.Int("lineNo", lineNo),
			zap.Error(err),
		)
		return nil, false
	}

	return compiledRegex, negate
}

// normalizePath normalizes the path for matching.
func normalizePath(path string) string {
	// Ensure paths use forward slashes
	path = filepath.ToSlash(path)

	// Add trailing slash for directories if not present
	if info, err := os.Stat(path); err == nil && info.IsDir() && !strings.HasSuffix(path, "/") {
		path += "/"
	}

	return path
}
