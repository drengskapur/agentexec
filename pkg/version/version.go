// Package version provides version information for the AgentExec CLI tool.
package version

import (
	"fmt"
	"runtime"
)

// These variables are populated at build time using -ldflags.
// Example:
// go build -ldflags "-X 'agentexec/pkg/version.Version=1.2.3' -X 'agentexec/pkg/version.Commit=abcdefg' -X 'agentexec/pkg/version.BuildTime=2024-04-27T15:04:05Z'"
var (
	Version   = "dev"     // Semantic version of the application
	Commit    = "none"    // Git commit hash
	BuildTime = "unknown" // Build timestamp
)

// Info contains comprehensive version information.
type Info struct {
	Version   string // Semantic version
	GitCommit string // Git commit hash
	BuildTime string // Build timestamp
	GoVersion string // Go runtime version
	Platform  string // OS and architecture
}

// Get returns the current version information.
func Get() Info {
	return Info{
		Version:   Version,
		GitCommit: Commit,
		BuildTime: BuildTime,
		GoVersion: runtime.Version(),
		Platform:  fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
	}
}

// String returns the version information in a standard, single-line format.
// Example Output:
// agentexec version 1.2.3 (commit: abcdefg) built at 2024-04-27T15:04:05Z with go1.20.4 on linux/amd64
func (i Info) String() string {
	return fmt.Sprintf(
		"agentexec version %s (commit: %s) built at %s with %s on %s",
		i.Version,
		i.GitCommit,
		i.BuildTime,
		i.GoVersion,
		i.Platform,
	)
}
