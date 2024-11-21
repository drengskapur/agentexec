package main

import (
	"log"

	"omnivex/pkg/combine"

	"go.uber.org/zap"
)

// main initializes the logger and starts the combine execution process.
func main() {
	// Initialize the logger
	logger, err := zap.NewProduction()
	if err != nil {
		// Fallback to standard library logging if zap initialization fails
		log.Fatalf("Failed to initialize logger: %v", err)
	}
	// Ensure the logger's buffer is flushed and handle any potential error.
	defer func() {
		if syncErr := logger.Sync(); syncErr != nil {
			log.Printf("Logger sync failed: %v", syncErr)
		}
	}()

	// Execute the main logic with the logger
	if err := combine.Execute(logger); err != nil {
		logger.Fatal("omnivex execution failed", zap.Error(err))
	}
}
