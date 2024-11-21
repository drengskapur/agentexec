package logging

import (
	"go.uber.org/zap"
)

// Logger is the global logger instance
var Logger *zap.Logger

func Setup(debug bool, appName, appVersion string) error {
	var err error
	var cfg zap.Config

	if debug {
		cfg = zap.NewDevelopmentConfig()
	} else {
		cfg = zap.NewProductionConfig()
	}

	// Add default fields
	cfg.InitialFields = map[string]interface{}{
		"appName":    appName,
		"appVersion": appVersion,
	}

	Logger, err = cfg.Build()
	if err != nil {
		Logger = zap.NewExample()
		return err
	}

	zap.ReplaceGlobals(Logger)
	return nil
}
