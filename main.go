package main

import (
	"log"
	"omnivex/cmd"
	"os"
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"golang.org/x/term"
)

func main() {
	// Default to plain text output
	outputFormat := "text"

	// Check for an environment variable to override the output format
	if envFormat := os.Getenv("OMNIVEX_LOG_FORMAT"); envFormat != "" {
		outputFormat = envFormat
	}

	var logger *zap.Logger
	var err error

	// Create the logger based on the chosen format
	switch strings.ToLower(outputFormat) {
	case "json":
		logger, err = zap.NewProduction(zap.Fields(
			zap.String("appName", "Omnivex"),
			zap.String("appVersion", "1.0.0"),
		))
	case "text", "": // Treat empty string as text
		config := zap.NewDevelopmentConfig()
		config.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder // Add color for text output
		logger, err = config.Build()
	default:
		log.Fatalf("Invalid log format: %s. Supported formats: json, text", outputFormat)
	}

	if err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}
	defer func() {
		if term.IsTerminal(int(os.Stderr.Fd())) || isRegularFile(os.Stderr) {
			if syncErr := logger.Sync(); syncErr != nil {
				lowerErr := strings.ToLower(syncErr.Error())
				if !strings.Contains(lowerErr, "invalid argument") {
					log.Printf("Logger sync failed: %v", syncErr)
				}
			}
		}
	}()

	logger.Info("Omnivex application started", zap.String("logFormat", outputFormat)) // Log the format

	if err := cmd.Execute(logger); err != nil {
		logger.Fatal("Omnivex execution failed", zap.Error(err))
	}

	logger.Info("Omnivex application finished successfully")
}

// isRegularFile checks if the given file is a regular file.
func isRegularFile(f *os.File) bool {
	fileInfo, err := f.Stat()
	if err != nil {
		return false
	}
	return fileInfo.Mode().IsRegular()
}
