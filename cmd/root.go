// cmd/root.go
package cmd

import (
	"context"

	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

type contextKey string

const loggerKey contextKey = "logger"

// RootCmd is the base command when called without any subcommands
var RootCmd = &cobra.Command{
	Use:   "omnivex",
	Short: "Omnivex is a CLI tool for combining files",
	Long:  `Omnivex combines multiple files into a single text file.`,
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute(logger *zap.Logger) error {
	// Create a context with the logger
	ctx := context.WithValue(context.Background(), loggerKey, logger)
	// Set the context to RootCmd
	RootCmd.SetContext(ctx)
	return RootCmd.Execute()
}

func init() {
	RootCmd.AddCommand(combineCmd)
}
