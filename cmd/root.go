// File: cmd/root.go
package cmd

import (
	"context"

	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

// contextKey is a custom type to avoid context key collisions.
type contextKey string

const (
	// loggerKey is the context key for the Zap logger.
	loggerKey contextKey = "logger"
)

// RootCmd represents the base command when called without any subcommands.
var RootCmd = &cobra.Command{
	Use:   "agentexec",
	Short: "AgentExec is a multipurpose CLI tool",
	Long:  `AgentExec is a command-line interface tool designed to perform various tasks.`,
}

// Execute initializes the root command with the provided logger and executes it.
// It sets up the context to include the logger for use in subcommands.
func Execute(logger *zap.Logger) error {
	// Create a background context and attach the logger to it.
	ctx := context.WithValue(context.Background(), loggerKey, logger)

	// Assign the context to the root command.
	RootCmd.SetContext(ctx)

	// Execute the root command, which parses flags and runs the appropriate subcommand.
	return RootCmd.Execute()
}

func init() {
	// Initialize and add subcommands to the root command.
	// Ensure that combineCmd and versionCmd are properly defined in their respective files.
	RootCmd.AddCommand(combineCmd)
	RootCmd.AddCommand(versionCmd)
}
