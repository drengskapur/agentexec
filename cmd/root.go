package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

// RootCmd is the base command when called without any subcommands.
var RootCmd = &cobra.Command{
	Use:   "omnivex",
	Short: "Omnivex is a CLI tool for combining files",
	Long:  `Omnivex combines multiple files into a single output, designed for workflows like ChatGPT input preparation.`,
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() {
	if err := RootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
