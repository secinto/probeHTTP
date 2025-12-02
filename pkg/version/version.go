package version

import (
	"fmt"
	"runtime"
)

var (
	// Version is the semantic version (set via ldflags at build time)
	Version = "dev"
	// GitCommit is the git SHA (set via ldflags at build time)
	GitCommit = "unknown"
	// BuildDate is the build date (set via ldflags at build time)
	BuildDate = "unknown"
)

// GetVersion returns the full version information
func GetVersion() string {
	return fmt.Sprintf("probeHTTP %s (commit: %s, built: %s, go: %s)",
		Version, GitCommit, BuildDate, runtime.Version())
}

// GetShortVersion returns just the version number
func GetShortVersion() string {
	return Version
}
