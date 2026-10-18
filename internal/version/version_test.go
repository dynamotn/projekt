package version

import (
	"runtime"
	"testing"
)

func TestGetVersionStr(t *testing.T) {
	if got := GetVersionStr(); got == "" {
		t.Error("GetVersionStr() returned empty string")
	}

	// Stamped by ldflags, the stamp wins.
	original := version
	version = "v9.9.9"
	t.Cleanup(func() { version = original })
	if got := GetVersionStr(); got != "v9.9.9" {
		t.Errorf("GetVersionStr() = %v, want the stamped version", got)
	}

	// Nothing stamped: never empty, and never a stale constant.
	version = ""
	if got := GetVersionStr(); got == "" || got == "v1.1.0" {
		t.Errorf("GetVersionStr() = %q, want a module or development version", got)
	}
}

func TestGetBuildInfo(t *testing.T) {
	info := GetBuildInfo()

	if info.Version == "" {
		t.Error("BuildInfo.Version is empty")
	}
	if info.GoVersion != runtime.Version() {
		t.Errorf("BuildInfo.GoVersion = %v, want %v", info.GoVersion, runtime.Version())
	}
}

func TestBuildInfoFields(t *testing.T) {
	// Set some test values
	originalVersion := version
	originalCommit := gitCommit
	originalTreeState := gitTreeState
	originalBuildTime := buildTime

	version = "v1.2.3"
	gitCommit = "abc123"
	gitTreeState = "clean"
	buildTime = "2024-01-01T00:00:00Z"

	defer func() {
		version = originalVersion
		gitCommit = originalCommit
		gitTreeState = originalTreeState
		buildTime = originalBuildTime
	}()

	info := GetBuildInfo()

	if info.Version != "v1.2.3" {
		t.Errorf("BuildInfo.Version = %v, want v1.2.3", info.Version)
	}
	if info.GitCommit != "abc123" {
		t.Errorf("BuildInfo.GitCommit = %v, want abc123", info.GitCommit)
	}
	if info.GitTreeState != "clean" {
		t.Errorf("BuildInfo.GitTreeState = %v, want clean", info.GitTreeState)
	}
	if info.BuildTime != "2024-01-01T00:00:00Z" {
		t.Errorf("BuildInfo.BuildTime = %v, want 2024-01-01T00:00:00Z", info.BuildTime)
	}
}
