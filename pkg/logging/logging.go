package logging

import (
	"go.uber.org/zap"
)

// Logger is the global logger instance
var Logger *zap.Logger

// Setup initializes the global logger
func Setup(debug bool) error {
	var err error
	if debug {
		Logger, err = zap.NewDevelopment() // Human-readable logs
	} else {
		Logger, err = zap.NewProduction() // JSON logs optimized for production
	}
	if err != nil {
		return err
	}
	zap.ReplaceGlobals(Logger) // Set as global logger for zap.L() and zap.S()
	return nil
}

// Close flushes any buffered log entries
func Close() {
	if Logger != nil {
		_ = Logger.Sync()
	}
}
