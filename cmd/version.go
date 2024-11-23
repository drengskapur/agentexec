// File: cmd/version.go
package cmd

import (
	"fmt"

	"agentexec/pkg/version"

	"github.com/spf13/cobra"
)

// versionCmd represents the version command.
// It displays the current version of the AgentExec application.
// The --short flag allows users to retrieve a concise version string.
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Display the version of AgentExec",
	Long:  `Display the current version information of the AgentExec CLI tool.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Retrieve the value of the --short flag
		short, err := cmd.Flags().GetBool("short")
		if err != nil {
			return fmt.Errorf("error reading flags: %w", err)
		}

		// Fetch version information
		v := version.Get()

		if short {
			// If --short is provided, print only the version number
			fmt.Println(v.Version)
		} else {
			// Otherwise, print the full version information
			fmt.Println(v.String())
		}

		return nil
	},
}

func init() {
	// Define the --short flag for the version command
	versionCmd.Flags().BoolP("short", "s", false, "Print the version number only")

	// Add the version command to the root command
	RootCmd.AddCommand(versionCmd)
}
