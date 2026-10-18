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

import (
	"runtime"
	"runtime/debug"
)

// devVersion is what a build nothing stamped reports.
const devVersion = "v0.0.0-dev"

var (
	version      = ""
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

// GetVersionStr returns the version string.
//
// A release build stamps it with ldflags. A `go install module@version`
// build does not, but the Go toolchain records the module version, so that
// is used next; a plain `go build` from a checkout gets a development
// version rather than a stale constant nobody updates.
func GetVersionStr() string {
	if version != "" {
		return version
	}
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}
	return devVersion
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
