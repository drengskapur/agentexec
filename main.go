package main

import (
	"log"

	"omnivex/pkg/combine"

	"go.uber.org/zap"
)

func main() {
	// Initialize the Zap logger for structured logging
	logger, err := zap.NewProduction()
	if err != nil {
		// Fallback to standard library logging if Zap initialization fails
		log.Fatalf("Failed to initialize logger: %v", err)
	}
	defer func() {
		if syncErr := logger.Sync(); syncErr != nil {
			log.Printf("Logger sync failed: %v", syncErr)
		}
	}()

	// Execute the main logic with the logger passed in
	if err := combine.Execute(logger); err != nil { // Pass logger here
		logger.Fatal("omnivex execution failed", zap.Error(err))
	}
}
