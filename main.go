package main

import (
	"log"
	"omnivex/cmd"
	"os"
	"strings"

	"go.uber.org/zap"
	"golang.org/x/term"
)

func main() {
	logger, err := zap.NewProduction(zap.Fields(
		zap.String("appName", "Omnivex"),
		zap.String("appVersion", "1.0.0"),
	))
	if err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}
	defer logger.Sync() // Still call Sync, but handle potential errors differently

	// Execute the root command
	if err := cmd.Execute(logger); err != nil {
		logger.Fatal("omnivex execution failed", zap.Error(err))
	}

	// Check if stderr is a terminal or a regular file before attempting to sync.
	if term.IsTerminal(int(os.Stderr.Fd())) || isRegularFile(os.Stderr) {
		if syncErr := logger.Sync(); syncErr != nil {
			lowerErr := strings.ToLower(syncErr.Error())
			if !strings.Contains(lowerErr, "invalid argument") { // Still check for other errors
				log.Printf("Logger sync failed: %v", syncErr)
			}
		}
	}
}

// isRegularFile checks if the given file is a regular file.
func isRegularFile(f *os.File) bool {
	fileInfo, err := f.Stat()
	if err != nil {
		return false // Assume not a regular file if we can't get the file info
	}
	return fileInfo.Mode().IsRegular()
}
