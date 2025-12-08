// Package version provides build and version information.
//
// It exposes version, git commit, build time, and other build metadata
// that can be set at build time using ldflags.
//
// Example usage:
//
//	info := version.GetBuildInfo()
//	fmt.Printf("Version: %s\n", info.Version)
//	fmt.Printf("Git Commit: %s\n", info.GitCommit)
package version

import "runtime"

var (
	version      = "v1.1.0"
	gitCommit    = ""
	gitTreeState = ""
	buildTime    = ""
)

// BuildInfo contains version and build information
type BuildInfo struct {
	Version      string
	GitCommit    string
	GitTreeState string
	GoVersion    string
	BuildTime    string
}

// GetVersionStr returns the version string
func GetVersionStr() string {
	return version
}

// GetBuildInfo returns the build information
func GetBuildInfo() BuildInfo {
	return BuildInfo{
		Version:      GetVersionStr(),
		GitCommit:    gitCommit,
		GitTreeState: gitTreeState,
		GoVersion:    runtime.Version(),
		BuildTime:    buildTime,
	}
}
