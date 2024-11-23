// File: pkg/combine/binary.go
package combine

import (
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
)

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

// isCommonBinaryExtension checks if the file has a known binary extension
func isCommonBinaryExtension(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	return BinaryExtensions[ext]
}
