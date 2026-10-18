package cli

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestCommand_ResolvesToAnAbsolutePath(t *testing.T) {
	dir := t.TempDir()
	tool := filepath.Join(dir, "sometool")
	if err := os.WriteFile(tool, []byte("#!/bin/sh\necho ok\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)

	cmd := Command("sometool", "arg")
	if cmd.Err != nil {
		t.Fatalf("Command().Err = %v", cmd.Err)
	}
	if cmd.Path != tool {
		t.Errorf("Path = %q, want %q", cmd.Path, tool)
	}
	if len(cmd.Args) != 2 || cmd.Args[1] != "arg" {
		t.Errorf("Args = %v, want the arguments kept", cmd.Args)
	}
	out, err := cmd.Output()
	if err != nil || string(out) != "ok\n" {
		t.Errorf("Output() = %q, %v", out, err)
	}
}

func TestCommand_MissingToolFailsWhenRun(t *testing.T) {
	t.Setenv("PATH", t.TempDir())

	cmd := Command("no-such-tool-anywhere")
	if cmd.Err == nil {
		t.Fatal("Command().Err = nil, want the lookup error")
	}
	if err := cmd.Run(); err == nil {
		t.Error("Run() error = nil, want it to fail")
	}
}

func TestCommand_RefusesARelativeLookup(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "planted"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)
	t.Setenv("PATH", ".")

	cmd := Command("planted")
	if cmd.Err == nil {
		t.Fatal("a tool found through a relative PATH entry must not run")
	}
	if err := cmd.Run(); !errors.Is(err, cmd.Err) && err == nil {
		t.Errorf("Run() error = %v, want the lookup error", err)
	}
}
