package folderutil

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"gitlab.com/dynamo-tools/projekt/pkg/cli"
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

func TestExecInFolders_DefaultOutputUnchanged(t *testing.T) {
	a, _ := execFolders(t)
	gone := filepath.Join(filepath.Dir(a), "gone")

	var out bytes.Buffer
	err := ExecInFolders(&out, ExecOptions{Command: []string{"sh", "-c", "echo hi; echo oops >&2; exit 2"}})
	if err == nil {
		t.Fatal("ExecInFolders() error = nil, want a failure")
	}

	want := "alpha: exit 2\n  hi\n  oops\nbeta: exit 2\n  hi\n  oops\ngone: missing " + gone + "\n"
	if out.String() != want {
		t.Errorf("output = %q, want %q", out.String(), want)
	}
}

func TestExecInFolders_JSON(t *testing.T) {
	a, b := execFolders(t)

	var out bytes.Buffer
	err := ExecInFolders(&out, ExecOptions{
		// Fails only in beta, where there is no marker file.
		Command: []string{"sh", "-c", "echo hi; test -f marker || exit 3"},
		Output:  cli.OutputJSON,
	})
	if err == nil || !strings.Contains(err.Error(), "1 of") {
		t.Fatalf("ExecInFolders() error = %v, want one folder failed", err)
	}

	var got []map[string]any
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("output %q is not JSON: %v", out.String(), err)
	}
	if len(got) != 3 {
		t.Fatalf("got %d objects, want 3: %v", len(got), got)
	}
	if got[2]["exit"] != float64(-1) || got[2]["output"] != "missing" {
		t.Errorf("missing folder object = %v, want exit -1 and output missing", got[2])
	}
	for i, want := range []struct {
		name, path string
		exit       float64
	}{{"alpha", a, 0}, {"beta", b, 3}} {
		o := got[i]
		if len(o) != 5 {
			t.Errorf("object %d = %v, want exactly name, path, exit, output, seconds", i, o)
		}
		if o["name"] != want.name || o["path"] != want.path || o["exit"] != want.exit || o["output"] != "hi\n" {
			t.Errorf("object %d = %v, want %+v and output %q", i, o, want, "hi\n")
		}
		if s, ok := o["seconds"].(float64); !ok || s < 0 {
			t.Errorf("object %d seconds = %v, want a number >= 0", i, o["seconds"])
		}
	}
}

func TestExecInFolders_JSONMissingCommand(t *testing.T) {
	execFolders(t)

	var out bytes.Buffer
	_ = ExecInFolders(&out, ExecOptions{Command: []string{"no-such-program"}, Tags: []string{"oss"}, Output: cli.OutputJSON})

	var got []struct {
		Exit   int
		Output string
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("output %q is not JSON: %v", out.String(), err)
	}
	if len(got) != 1 || got[0].Exit != -1 || !strings.Contains(got[0].Output, "no-such-program") {
		t.Errorf("got %+v, want exit -1 and the start error as output", got)
	}
}

func TestExecInFolders_JSONEmptySelectionAndFlags(t *testing.T) {
	execFolders(t)

	var out bytes.Buffer
	if err := ExecInFolders(&out, ExecOptions{Command: []string{"true"}, Tags: []string{"nope"}, Output: cli.OutputJSON}); err != nil {
		t.Fatal(err)
	}
	if strings.TrimSpace(out.String()) != "[]" {
		t.Errorf("output = %q, want []", out.String())
	}

	for name, o := range map[string]ExecOptions{
		"table":     {Output: cli.OutputTable},
		"quiet":     {Output: cli.OutputJSON, Quiet: true},
		"no-output": {Output: cli.OutputJSON, NoOutput: true},
	} {
		o.Command = []string{"true"}
		if err := ExecInFolders(&out, o); err == nil {
			t.Errorf("%s: error = nil, want one", name)
		}
	}
}

func TestExecInFolders_JSONDryRunAndEscaping(t *testing.T) {
	a, _ := execFolders(t)

	var out bytes.Buffer
	err := ExecInFolders(&out, ExecOptions{Command: []string{"touch", "ran"}, Tags: []string{"oss"}, DryRun: true, Output: cli.OutputJSON})
	if err != nil {
		t.Fatal(err)
	}
	var got []map[string]any
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("output %q is not JSON: %v", out.String(), err)
	}
	if len(got) != 1 || len(got[0]) != 5 || got[0]["exit"] != float64(0) || got[0]["output"] != "" || got[0]["seconds"] != float64(0) {
		t.Errorf("got %v, want one five-key object with exit 0, no output, 0 seconds", got)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(a), "beta", "ran")); err == nil {
		t.Error("the command ran during a dry run")
	}

	out.Reset()
	if err := ExecInFolders(&out, ExecOptions{Command: []string{"echo", "<b>&"}, Tags: []string{"oss"}, Output: cli.OutputJSON}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), `"<b>&\n"`) {
		t.Errorf("output = %q, want <b>& unescaped", out.String())
	}
}

func TestExecInFolders_Timeout(t *testing.T) {
	execFolders(t)
	pids := filepath.Join(t.TempDir(), "pids")
	t.Cleanup(func() {
		data, _ := os.ReadFile(pids)
		for _, line := range strings.Fields(string(data)) {
			if pid, err := strconv.Atoi(line); err == nil {
				_ = syscall.Kill(-pid, syscall.SIGKILL)
			}
		}
	})

	var out bytes.Buffer
	start := time.Now()
	err := ExecInFolders(&out, ExecOptions{
		Command: []string{"sh", "-c", `echo $$ >> "$1"; sleep 30`, "sh", pids},
		Timeout: 200 * time.Millisecond,
	})
	if took := time.Since(start); took > 5*time.Second {
		t.Errorf("ExecInFolders() took %s, want it stopped soon after the timeout", took)
	}
	if err == nil {
		t.Error("a command that timed out must fail exec")
	}
	for _, want := range []string{"alpha: timed out", "beta: timed out"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("output = %q, want it to contain %q", out.String(), want)
		}
	}

	// In JSON, the output says so.
	out.Reset()
	_ = ExecInFolders(&out, ExecOptions{
		Command: []string{"sh", "-c", `echo $$ >> "$1"; echo started; sleep 30`, "sh", pids},
		Timeout: 200 * time.Millisecond, Output: cli.OutputJSON, Tags: []string{"oss"},
	})
	var records []execRecord
	if err := json.Unmarshal(out.Bytes(), &records); err != nil || len(records) != 1 {
		t.Fatalf("not one JSON record: %v\n%s", err, out.String())
	}
	if rec := records[0]; rec.Exit != -1 || !strings.Contains(rec.Output, "started") || !strings.Contains(rec.Output, "timed out") {
		t.Errorf("record = %+v, want exit -1 and the output followed by timed out", rec)
	}
}

func TestExecInFolders_FailFast(t *testing.T) {
	root := t.TempDir()
	var folders []lazypath.Folder
	for _, name := range []string{"first", "second", "third"} {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(path, 0o755); err != nil {
			t.Fatal(err)
		}
		folders = append(folders, lazypath.Folder{Path: path})
	}
	if err := os.WriteFile(filepath.Join(root, "first", "fail"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	lazypath.SetTestConfig(lazypath.Config{Folders: folders})
	t.Cleanup(lazypath.ResetTestConfig)

	var out bytes.Buffer
	err := ExecInFolders(&out, ExecOptions{
		Command:  []string{"sh", "-c", "touch ran; test ! -f fail"},
		Forks:    1,
		FailFast: true,
	})
	if err == nil {
		t.Error("the first folder failed, exec must too")
	}
	if _, statErr := os.Stat(filepath.Join(root, "third", "ran")); statErr == nil {
		t.Error("third ran after the first failure")
	}
	for _, want := range []string{"first: exit 1", "second: not started", "third: not started"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("output = %q, want it to contain %q", out.String(), want)
		}
	}
}
