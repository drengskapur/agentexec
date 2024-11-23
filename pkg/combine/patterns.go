// File: pkg/combine/patterns.go
package combine

import (
	"regexp"
	"strings"
)

// Precompiled regular expressions used in pattern parsing.
var (
	DoubleStarMiddlePattern      = regexp.MustCompile(`/\*\*/`)
	DoubleStarTrailingPattern    = regexp.MustCompile(`/\*\*$`)
	DoubleStarLeadingPattern     = regexp.MustCompile(`^\*\*/`)
	SingleStarReplacementPattern = regexp.MustCompile(`\*`)
	DirectoryEndPattern          = regexp.MustCompile(`/$`)
	RootRelativePattern          = regexp.MustCompile(`^/`)
)

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
	pattern = DoubleStarMiddlePattern.ReplaceAllString(pattern, `(/|/.+/)`)
	pattern = DoubleStarTrailingPattern.ReplaceAllString(pattern, `(/.*)?`)
	pattern = DoubleStarLeadingPattern.ReplaceAllString(pattern, `(.*/)?`)
	return pattern
}

// wildcardToRegex converts wildcard patterns '*' and '?' to regex equivalents.
func wildcardToRegex(pattern string) string {
	pattern = SingleStarReplacementPattern.ReplaceAllString(pattern, `[^/]*`)
	pattern = strings.ReplaceAll(pattern, "?", ".")
	return pattern
}

// anchorPattern anchors the regex pattern to match the entire path.
func anchorPattern(pattern string, originalPattern string) string {
	if DirectoryEndPattern.MatchString(originalPattern) {
		pattern = pattern + "(/.*)?$"
	} else {
		pattern = pattern + "(|/.*)?$"
	}

	if RootRelativePattern.MatchString(originalPattern) {
		return "^" + pattern
	}
	return "^(|.*/)" + pattern
}
