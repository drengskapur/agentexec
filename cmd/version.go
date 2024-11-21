// cmd/version.go
package cmd

import (
	"fmt"
	"os"

	"omnivex/pkg/version"

	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

// versionCmd represents the version command
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Display the version of Omnivex",
	Long:  `All software has versions. This is Omnivex's.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Retrieve the logger from the context
		logger, ok := cmd.Context().Value(loggerKey).(*zap.Logger)
		if !ok || logger == nil {
			// If logger is not available, log to stderr and exit
			fmt.Fprintln(os.Stderr, "Logger not initialized")
			os.Exit(1)
		}

		// Fetch version information
		v := version.Get()

		// Display version information to the user
		fmt.Println(v.String())

		// Log that the version command was executed
		logger.Debug("Executed version command", zap.String("version", v.Version), zap.String("commit", v.GitCommit))
	},
}

func init() {
	RootCmd.AddCommand(versionCmd)
}
