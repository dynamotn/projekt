package folderutil

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gitlab.com/dynamo-tools/projekt/pkg/lazypath"
)

// execFolders configures three folders, two of which exist.
func execFolders(t *testing.T) (a, b string) {
	t.Helper()

	root := t.TempDir()
	a = filepath.Join(root, "alpha")
	b = filepath.Join(root, "beta")
	for _, path := range []string{a, b} {
		if err := os.MkdirAll(path, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(a, "marker"), []byte("here\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	lazypath.SetTestConfig(lazypath.Config{Folders: []lazypath.Folder{
		{Path: a, Tags: []string{"work"}},
		{Path: b, Tags: []string{"oss"}},
		{Path: filepath.Join(root, "gone"), Tags: []string{"work"}},
	}})
	t.Cleanup(lazypath.ResetTestConfig)

	return a, b
}

func TestExecInFolders(t *testing.T) {
	execFolders(t)

	var out bytes.Buffer
	if err := ExecInFolders(&out, ExecOptions{Command: []string{"pwd"}}); err != nil {
		t.Fatalf("ExecInFolders() error = %v", err)
	}

	output := out.String()
	for _, want := range []string{"alpha: ok", "beta: ok", "gone: missing"} {
		if !strings.Contains(output, want) {
			t.Errorf("output = %q, want it to contain %q", output, want)
		}
	}
	// Reported in configuration order, so two runs can be compared.
	if strings.Index(output, "alpha") > strings.Index(output, "beta") {
		t.Errorf("output = %q, want configuration order", output)
	}
}

func TestExecInFolders_ReportsAFailure(t *testing.T) {
	execFolders(t)

	var out bytes.Buffer
	err := ExecInFolders(&out, ExecOptions{Command: []string{"test", "-f", "marker"}})
	if err == nil {
		t.Fatal("ExecInFolders() error = nil, want an error when the command failed somewhere")
	}
	if !strings.Contains(err.Error(), "1 of 3") {
		t.Errorf("error = %v, want it to count the failures", err)
	}

	output := out.String()
	if !strings.Contains(output, "alpha: ok") {
		t.Errorf("output = %q, want alpha to pass", output)
	}
	if !strings.Contains(output, "beta: exit 1") {
		t.Errorf("output = %q, want beta to report its exit code", output)
	}
}

func TestExecInFolders_Quiet(t *testing.T) {
	execFolders(t)

	var out bytes.Buffer
	// Quiet turns it into a filter: only what answers no.
	_ = ExecInFolders(&out, ExecOptions{Command: []string{"test", "-f", "marker"}, Quiet: true})

	output := out.String()
	if strings.Contains(output, "alpha") {
		t.Errorf("output = %q, want the passing folder left out", output)
	}
	if !strings.Contains(output, "beta") {
		t.Errorf("output = %q, want the failing folder", output)
	}
	if strings.Contains(output, "missing") {
		t.Errorf("output = %q, want a missing folder left out too", output)
	}
}

func TestExecInFolders_Output(t *testing.T) {
	a, _ := execFolders(t)

	var out bytes.Buffer
	if err := ExecInFolders(&out, ExecOptions{Command: []string{"pwd"}, Tags: []string{"oss"}}); err != nil {
		t.Fatalf("ExecInFolders() error = %v", err)
	}
	if strings.Contains(out.String(), a) {
		t.Errorf("output = %q, want only the oss folder", out.String())
	}

	// The command's own output is indented under its folder.
	out.Reset()
	if err := ExecInFolders(&out, ExecOptions{Command: []string{"echo", "hello"}, Tags: []string{"oss"}}); err != nil {
		t.Fatalf("ExecInFolders() error = %v", err)
	}
	if !strings.Contains(out.String(), "  hello") {
		t.Errorf("output = %q, want the command output indented", out.String())
	}

	// And --no-output keeps the verdict alone.
	out.Reset()
	if err := ExecInFolders(&out, ExecOptions{Command: []string{"echo", "hello"}, Tags: []string{"oss"}, NoOutput: true}); err != nil {
		t.Fatalf("ExecInFolders() error = %v", err)
	}
	if strings.Contains(out.String(), "hello") {
		t.Errorf("output = %q, want no command output", out.String())
	}
}

func TestExecInFolders_DryRun(t *testing.T) {
	a, _ := execFolders(t)

	var out bytes.Buffer
	if err := ExecInFolders(&out, ExecOptions{Command: []string{"rm", "-rf", "."}, DryRun: true}); err != nil {
		t.Fatalf("ExecInFolders() error = %v", err)
	}
	if !strings.Contains(out.String(), "[DRY RUN]") || !strings.Contains(out.String(), "rm -rf .") {
		t.Errorf("output = %q, want the plan", out.String())
	}
	// Nothing ran: a dry run that deleted the folders would be quite a bug.
	if _, err := os.Stat(filepath.Join(a, "marker")); err != nil {
		t.Errorf("the dry run touched the folder: %v", err)
	}
}

func TestExecInFolders_Errors(t *testing.T) {
	execFolders(t)

	var out bytes.Buffer
	if err := ExecInFolders(&out, ExecOptions{}); err == nil {
		t.Error("ExecInFolders() with no command error = nil, want an error")
	}

	// A command that is not on PATH fails per folder rather than blowing up.
	out.Reset()
	if err := ExecInFolders(&out, ExecOptions{Command: []string{"definitely-not-a-command-xyz"}}); err == nil {
		t.Error("ExecInFolders() with an unknown command error = nil, want an error")
	}
}

func TestExecForks(t *testing.T) {
	if got := execForks(0, 10); got != DefaultSyncForks {
		t.Errorf("execForks(0, 10) = %d, want the default", got)
	}
	if got := execForks(100, 3); got != 3 {
		t.Errorf("execForks(100, 3) = %d, want 3, one per folder", got)
	}
	if got := execForks(2, 10); got != 2 {
		t.Errorf("execForks(2, 10) = %d, want 2", got)
	}
}
