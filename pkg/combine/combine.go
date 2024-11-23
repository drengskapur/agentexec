// File: pkg/combine/combine.go
package combine

import (
	"go.uber.org/zap"
)

// ExecuteWithArgs initiates the combine process with the provided arguments and logger.
func ExecuteWithArgs(args Arguments, logger *zap.Logger) error {
	return executeProcess(args, logger)
}
