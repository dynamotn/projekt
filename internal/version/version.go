package version

import "runtime"

var (
	version      = "v1.1.0"
	gitCommit    = ""
	gitTreeState = ""
	buildTime    = ""
)

type BuildInfo struct {
	Version      string
	GitCommit    string
	GitTreeState string
	GoVersion    string
	BuildTime    string
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
		BuildTime:    buildTime,
	}
}
