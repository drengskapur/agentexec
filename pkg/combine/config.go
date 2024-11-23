// File: pkg/combine/config.go
package combine

// Arguments holds the configuration options for the file combining process.
type Arguments struct {
	Paths            []string // List of file or directory paths to be processed.
	Output           string   // Destination path for the combined output file.
	Tree             string   // Destination path for the tree structure output file.
	GlobalIgnoreFile string   // Optional path to a global .combineignore file for ignore patterns.
	MaxFileSizeKB    int      // Maximum size (in KB) of files to process; larger files are skipped.
	MaxWorkers       int      // Number of concurrent workers for processing files.
	IgnorePatterns   []string // Additional ignore patterns provided via command-line arguments.
	Verbose          bool     // If true, enables detailed logging, including skipped file information.
}

// FileContent represents the structured content of a single file.
type FileContent struct {
	Path    string // Relative file path to the file being processed.
	Content string // The formatted content of the file.
}

// CollectedFiles contains categorized lists of files discovered during processing.
type CollectedFiles struct {
	Regular []string // List of paths to regular (non-binary) files.
	Binary  []string // List of paths to binary files.
}
