package ignore

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"go.uber.org/zap"
)

// IgnorePattern encapsulates a compiled regular expression pattern,
// a negation flag, and metadata about the pattern's origin.
type IgnorePattern struct {
	Pattern *regexp.Regexp // Compiled regular expression for the pattern.
	Negate  bool           // Indicates if the pattern is a negation (starts with '!').
	Line    string         // Original pattern line.
	LineNo  int            // Line number in the source (1-based).
}

// GitIgnore represents a collection of ignore patterns.
type GitIgnore struct {
	Patterns []*IgnorePattern // List of compiled ignore patterns.
	logger   *zap.Logger      // Optional logger for debug information.
}

// NewGitIgnore initializes a GitIgnore instance with an optional logger.
func NewGitIgnore(logger *zap.Logger) *GitIgnore {
	if logger == nil {
		logger, _ = zap.NewProduction() // Use a production logger as default.
	}
	return &GitIgnore{
		Patterns: []*IgnorePattern{},
		logger:   logger,
	}
}

// LoadIgnoreFiles loads ignore patterns from local and global ignore files.
func LoadIgnoreFiles(localPath, globalPath string) (*GitIgnore, error) {
	gi := NewGitIgnore(nil) // Use a default logger if none is provided.

	// Load global ignore file if specified.
	if globalPath != "" {
		if err := gi.CompileIgnoreFile(globalPath); err != nil && !os.IsNotExist(err) {
			return nil, err
		}
	}

	// Load local ignore file if specified.
	if localPath != "" {
		if err := gi.CompileIgnoreFile(localPath); err != nil && !os.IsNotExist(err) {
			return nil, err
		}
	}

	return gi, nil
}

// CompileIgnoreLines compiles a set of ignore pattern lines and adds them to the GitIgnore instance.
func (gi *GitIgnore) CompileIgnoreLines(lines ...string) {
	for i, line := range lines {
		pattern, negate := parsePatternLine(line) // Remove lineNo
		if pattern != nil {
			gi.Patterns = append(gi.Patterns, &IgnorePattern{
				Pattern: pattern,
				Negate:  negate,
				Line:    line,
				LineNo:  i + 1, // 1-based line numbering.
			})
		}
	}
}

// CompileIgnoreFile reads an ignore file, parses its lines, and adds them to the GitIgnore instance.
func (gi *GitIgnore) CompileIgnoreFile(fpath string) error {
	content, err := os.ReadFile(fpath)
	if err != nil {
		gi.logger.Error("Failed to read ignore file", zap.String("filePath", fpath), zap.Error(err))
		return err
	}

	lines := strings.Split(string(content), "\n")
	gi.CompileIgnoreLines(lines...)
	gi.logger.Info("Compiled ignore patterns", zap.String("filePath", fpath), zap.Int("lineCount", len(lines)))
	return nil
}

// MatchesPath checks if a path matches any of the ignore patterns.
func (gi *GitIgnore) MatchesPath(path string) bool {
	matches, _ := gi.MatchesPathWithPattern(path)
	return matches
}

// MatchesPathWithPattern checks if a path matches any ignore pattern and returns
// the matched pattern if applicable.
func (gi *GitIgnore) MatchesPathWithPattern(path string) (bool, *IgnorePattern) {
	normalizedPath := normalizePath(path)

	var matchedPattern *IgnorePattern
	matches := false

	for _, pattern := range gi.Patterns {
		if pattern.Pattern.MatchString(normalizedPath) {
			matchedPattern = pattern
			if pattern.Negate {
				matches = false
			} else {
				matches = true
			}
		}
	}

	return matches, matchedPattern
}

// normalizePath converts OS-specific path separators to forward slashes.
func normalizePath(path string) string {
	return filepath.ToSlash(path)
}

// parsePatternLine processes a line from an ignore file into a compiled regex and a negation flag.
func parsePatternLine(line string) (*regexp.Regexp, bool) {
	trimmedLine := strings.TrimSpace(line)

	// Ignore empty lines and comments.
	if trimmedLine == "" || strings.HasPrefix(trimmedLine, "#") {
		return nil, false
	}

	// Check for negation.
	negate := false
	if strings.HasPrefix(trimmedLine, "!") {
		negate = true
		trimmedLine = strings.TrimPrefix(trimmedLine, "!")
	}

	// Handle escaped characters for `#` and `!`.
	if strings.HasPrefix(trimmedLine, "\\#") || strings.HasPrefix(trimmedLine, "\\!") {
		trimmedLine = trimmedLine[1:]
	}

	// Escape special characters and convert wildcards.
	escapedLine := escapeSpecialChars(trimmedLine)
	escapedLine = handleDoubleStarPatterns(escapedLine)
	escapedLine = wildcardToRegex(escapedLine)
	escapedLine = anchorPattern(escapedLine, trimmedLine)

	compiledRegex, err := regexp.Compile(escapedLine)
	if err != nil {
		return nil, false
	}

	return compiledRegex, negate
}

// escapeSpecialChars escapes regex special characters except for `*`, `?`, and `/`.
func escapeSpecialChars(pattern string) string {
	specialChars := `.+()|^$[]{}`
	for _, char := range specialChars {
		pattern = strings.ReplaceAll(pattern, string(char), `\`+string(char))
	}
	return pattern
}

// handleDoubleStarPatterns processes '**' patterns into regex equivalents.
func handleDoubleStarPatterns(pattern string) string {
	pattern = regexp.MustCompile(`/\*\*/`).ReplaceAllString(pattern, `(/|/.+/)`)
	pattern = regexp.MustCompile(`/\*\*$`).ReplaceAllString(pattern, `(/.*)?`)
	pattern = regexp.MustCompile(`^\*\*/`).ReplaceAllString(pattern, `(.*/)?`)
	return pattern
}

// wildcardToRegex converts `*` and `?` wildcards to regex equivalents.
func wildcardToRegex(pattern string) string {
	pattern = regexp.MustCompile(`\*`).ReplaceAllString(pattern, `[^/]*`)
	return strings.ReplaceAll(pattern, "?", ".")
}

// anchorPattern anchors the regex pattern to match the full path.
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
