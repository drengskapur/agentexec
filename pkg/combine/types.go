package combine

// Arguments holds the command-line arguments for the combine operation.
type Arguments struct {
	Directory     string   // The directory to process
	Output        string   // The output file for combined content
	Tree          string   // The output file for the directory tree structure
	Debug         string   // The debug log file
	IncludeFiles  []string // Specific files to include, overriding ignore rules
	MaxFileSizeKB int      // Maximum size of files to process (in KB)
	MaxWorkers    int      // Number of worker threads
	GlobalIgnore  string   // Path to the global ignore file
}

// FileContent holds the content of a file after processing.
type FileContent struct {
	Path    string // The file path
	Content string // The processed content of the file
}

// Constants
const (
	ChunkSize = 8192 // The size of chunks to read when processing files
)
