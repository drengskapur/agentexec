package main

import (
	"log"
	"os"
	"runtime/debug"

	"omnivex/cmd"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// createLogger creates and configures the application's logger
func createLogger(verbose bool) (*zap.Logger, error) {
	// Configure encoder
	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        "timestamp",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		FunctionKey:    zapcore.OmitKey,
		MessageKey:     "msg",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.CapitalLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.SecondsDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	// Create stdout syncer
	stdout := zapcore.AddSync(os.Stdout)

	// Determine log level based on verbose flag
	level := zap.InfoLevel
	if verbose {
		level = zap.DebugLevel
	}

	// Create console encoder and core
	consoleEncoder := zapcore.NewConsoleEncoder(encoderConfig)
	core := zapcore.NewCore(consoleEncoder, stdout, level)

	// Get build info for startup logging only
	buildInfo, _ := debug.ReadBuildInfo()

	// Create base logger
	logger := zap.New(core,
		zap.AddCaller(),
		zap.AddStacktrace(zapcore.ErrorLevel),
	)

	// Log startup information once
	logger.Debug("Starting Omnivex",
		zap.String("app_version", "1.0.0"),
		zap.String("go_version", buildInfo.GoVersion),
		zap.Int("pid", os.Getpid()),
		zap.Bool("verbose_mode", verbose),
	)

	// Return clean logger without default fields
	return logger, nil
}

func main() {
	// Parse verbose flag
	verbose := false
	for _, arg := range os.Args[1:] {
		if arg == "--verbose" || arg == "-v" {
			verbose = true
			break
		}
	}

	// Initialize logger
	logger, err := createLogger(verbose)
	if err != nil {
		log.Fatalf("Failed to initialize logger: %v", err)
	}
	defer func() {
		_ = logger.Sync()
	}()

	// Execute root command
	if err := cmd.Execute(logger); err != nil {
		logger.Error("Application execution failed",
			zap.Error(err),
			zap.String("command", os.Args[0]),
		)
		os.Exit(1)
	}
}
