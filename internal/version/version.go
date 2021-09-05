package version

var (
	version = "0.1"
)

type BuildInfo struct {
	Version string
}

func GetVersionStr() string {
	return version
}

func GetBuildInfo() BuildInfo {
	return BuildInfo{
		Version: GetVersionStr(),
	}
}
