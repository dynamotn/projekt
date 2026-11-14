package cli

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"
)

// fakeGit puts a stand-in git in a folder of its own and returns its path.
func fakeGit(t *testing.T, dir string) string {
	t.Helper()
	tool := filepath.Join(dir, "git")
	if err := os.WriteFile(tool, []byte("#!/bin/sh\necho ok\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	return tool
}

func TestCommand_ResolvesToAnAbsolutePath(t *testing.T) {
	dir := t.TempDir()
	tool := fakeGit(t, dir)
	t.Setenv("PATH", dir)

	cmd := Command("git", "arg")
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

	cmd := Command("git")
	if cmd.Err == nil {
		t.Fatal("Command().Err = nil, want the lookup error")
	}
	if err := cmd.Run(); err == nil {
		t.Error("Run() error = nil, want it to fail")
	}
}

func TestCommand_RefusesARelativeLookup(t *testing.T) {
	dir := t.TempDir()
	fakeGit(t, dir)
	t.Chdir(dir)
	t.Setenv("PATH", ".")

	cmd := Command("git")
	if cmd.Err == nil {
		t.Fatal("a tool found through a relative PATH entry must not run")
	}
	if err := cmd.Run(); err == nil {
		t.Error("Run() error = nil, want the lookup error")
	}
}

func TestCommand_RefusesAnUnknownTool(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "sometool"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)

	cmd := Command("sometool")
	if cmd.Err == nil {
		t.Fatal("only projekt's own tools may be started through Command")
	}
	if err := cmd.Run(); err == nil {
		t.Error("Run() error = nil, want it to fail")
	}
}

func TestCommandContext_KillsTheGroupOnTimeout(t *testing.T) {
	pidFile := filepath.Join(t.TempDir(), "pid")
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	cmd := CommandContext(ctx, "sh", "-c", `sleep 30 & echo $! > "$1"; wait`, "sh", pidFile)
	start := time.Now()
	err := cmd.Run()
	took := time.Since(start)

	data, readErr := os.ReadFile(pidFile)
	pid, convErr := strconv.Atoi(strings.TrimSpace(string(data)))
	if readErr != nil || convErr != nil {
		t.Fatalf("no background pid: %v %v", readErr, convErr)
	}
	// Whatever happens below, the sleep does not outlive the test.
	t.Cleanup(func() { _ = syscall.Kill(pid, syscall.SIGKILL) })

	if err == nil || ctx.Err() != context.DeadlineExceeded {
		t.Fatalf("Run() = %v with ctx.Err() = %v, want it stopped by the deadline", err, ctx.Err())
	}
	if took > 5*time.Second {
		t.Errorf("Run() took %s, want it stopped soon after the timeout", took)
	}
	// The background child went with its parent; give init a moment to reap it.
	deadline := time.Now().Add(3 * time.Second)
	for syscall.Kill(pid, 0) == nil {
		if time.Now().After(deadline) {
			t.Fatalf("background sleep %d still runs after the timeout", pid)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestCommandContext_RefusesAnUnknownTool(t *testing.T) {
	if err := CommandContext(context.Background(), "sometool").Run(); err == nil {
		t.Error("only projekt's own tools may be started through CommandContext")
	}
}
