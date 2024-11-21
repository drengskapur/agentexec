package main

import (
	"log"

	"omnivex/pkg/combine"

	"go.uber.org/zap"
)

func main() {
	logger, err := zap.NewProduction()
	if err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}
	defer func() {
		if syncErr := logger.Sync(); syncErr != nil && syncErr.Error() != "invalid argument" {
			log.Printf("Logger sync failed: %v", syncErr)
		}
	}()

	if err := combine.Execute(logger); err != nil {
		logger.Fatal("Execution failed", zap.Error(err))
	}
}
