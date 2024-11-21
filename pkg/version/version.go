package version

import (
	"fmt"
	"runtime"
)

var (
	// These will be populated by the compiler using -ldflags
	Version   = "dev"
	Commit    = "none"
	BuildTime = "unknown"
)

// Info contains version information
type Info struct {
	Version   string
	GitCommit string
	BuildTime string
	GoVersion string
	Platform  string
}

// Get returns the version info
func Get() Info {
	return Info{
		Version:   Version,
		GitCommit: Commit,
		BuildTime: BuildTime,
		GoVersion: runtime.Version(),
		Platform:  fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH),
	}
}

// String returns version information as a string
func (i Info) String() string {
	return fmt.Sprintf("Version: %s\nGit Commit: %s\nBuild Time: %s\nGo Version: %s\nPlatform: %s",
		i.Version,
		i.GitCommit,
		i.BuildTime,
		i.GoVersion,
		i.Platform,
	)
}
