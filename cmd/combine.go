package cmd

import (
	"log"

	"omnivex/pkg/combine"

	"go.uber.org/zap"
)

// Run initializes the logger and starts the combine execution process.
func Run() {
	// Initialize the logger
	logger, err := zap.NewProduction()
	if err != nil {
		// Fallback to standard logging if zap initialization fails
		log.Fatalf("Failed to initialize logger: %v", err)
	}
	defer logger.Sync() // Ensure the logger's buffer is flushed

	// Execute the combine package with the logger
	if err := combine.Execute(logger); err != nil {
		logger.Fatal("omnivex execution failed", zap.Error(err))
	}
}
