package version

import "runtime"

var (
	version      = "v0.2.0"
	gitCommit    = ""
	gitTreeState = ""
)

type BuildInfo struct {
	Version      string
	GitCommit    string
	GitTreeState string
	GoVersion    string
}

func GetVersionStr() string {
	return version
}

func GetBuildInfo() BuildInfo {
	return BuildInfo{
		Version:      GetVersionStr(),
		GitCommit:    gitCommit,
		GitTreeState: gitTreeState,
		GoVersion:    runtime.Version(),
	}
}
