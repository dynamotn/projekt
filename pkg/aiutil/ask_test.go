package aiutil

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/spf13/viper"

	"gitlab.com/dynamo-tools/projekt/pkg/lazypath"
)

// loadConfig points lazypath at a real configuration file holding content,
// the way a command starts.
func loadConfig(t *testing.T, content string) string {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	previous := lazypath.CfgFile
	lazypath.CfgFile = path
	lazypath.ResetTestConfig()
	viper.Reset()
	lazypath.InitConfig()
	t.Cleanup(func() {
		lazypath.CfgFile = previous
		lazypath.ResetTestConfig()
		viper.Reset()
	})
	return dir
}

// fakeAI writes a shell script with body and names it through PROJEKT_AI.
// TMPDIR is pointed at a folder of the test's own, returned, so what Ask
// leaves behind there can be checked.
func fakeAI(t *testing.T, body string) string {
	t.Helper()

	script := filepath.Join(t.TempDir(), "fake-ai")
	if err := os.WriteFile(script, []byte("#!/bin/sh\n"+body+"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PROJEKT_AI", script)
	tmp := t.TempDir()
	t.Setenv("TMPDIR", tmp)
	return tmp
}

func TestCommand_EnvironmentWins(t *testing.T) {
	loadConfig(t, "ai:\n  command: from-config\n")
	t.Setenv("PROJEKT_AI", "from-env -x")

	words, err := Command()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(words, "|") != "from-env|-x" {
		t.Errorf("Command() = %q, want [from-env -x]", words)
	}
}

func TestCommand_FromConfig(t *testing.T) {
	loadConfig(t, "ai:\n  command: \"'my ai' -p\"\n")
	t.Setenv("PROJEKT_AI", "")

	words, err := Command()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Join(words, "|") != "my ai|-p" {
		t.Errorf("Command() = %q, want [my ai -p]", words)
	}
}

func TestCommand_NotConfigured(t *testing.T) {
	loadConfig(t, "folders: []\n")
	t.Setenv("PROJEKT_AI", "")

	if _, err := Command(); !errors.Is(err, ErrNotConfigured) {
		t.Errorf("Command() error = %v, want ErrNotConfigured", err)
	}
}

func TestCommand_IncludedFileCannotSetIt(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "shared.yaml"), []byte("ai:\n  command: somebody-elses\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	loadConfig(t, "include:\n  - "+filepath.Join(dir, "shared.yaml")+"\n")
	t.Setenv("PROJEKT_AI", "")

	if words, err := Command(); !errors.Is(err, ErrNotConfigured) {
		t.Errorf("Command() = %q, %v, want ErrNotConfigured", words, err)
	}
}

func TestAsk_PromptOnStdinAnswerOnStdout(t *testing.T) {
	loadConfig(t, "")
	fakeAI(t, `cat > "$OUT"; printf 'answer\n'`)
	out := filepath.Join(t.TempDir(), "prompt")
	t.Setenv("OUT", out)

	prompt := "line one\nline two\n"
	answer, err := Ask(prompt, &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	if answer != "answer" {
		t.Errorf("Ask() = %q, want %q", answer, "answer")
	}
	if got, _ := os.ReadFile(out); string(got) != prompt {
		t.Errorf("the AI read %q, want %q", got, prompt)
	}
}

func TestAsk_RunsInAnEmptyFolderThatIsRemoved(t *testing.T) {
	loadConfig(t, "")
	tmp := fakeAI(t, `pwd; ls -A | wc -l`)

	answer, err := Ask("", &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(answer, "\n")
	if len(lines) != 2 {
		t.Fatalf("Ask() = %q, want the folder and its entry count", answer)
	}
	if strings.TrimSpace(lines[1]) != "0" {
		t.Errorf("the folder held %s entries, want 0", lines[1])
	}
	wd, _ := os.Getwd()
	if lazypath.SamePath(lines[0], wd) {
		t.Errorf("the AI ran in the caller's folder %s", wd)
	}
	if _, err := os.Stat(lines[0]); !os.IsNotExist(err) {
		t.Errorf("%s is still there after Ask: %v", lines[0], err)
	}
	if left, _ := os.ReadDir(tmp); len(left) != 0 {
		t.Errorf("Ask left %v in the temporary folder", left)
	}
}

func TestAsk_CleansTheAnswer(t *testing.T) {
	loadConfig(t, "")
	fakeAI(t, `printf '  \033[31mred\033[0m\000 ok\n\n'`)

	answer, err := Ask("", &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	if answer != "[31mred[0m ok" {
		t.Errorf("Ask() = %q, want %q", answer, "[31mred[0m ok")
	}

	fakeAI(t, `head -c 102400 /dev/zero | tr '\0' a`)
	answer, err = Ask("", &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	if len(answer) != 64<<10 {
		t.Errorf("an answer of 100 KiB came back as %d bytes, want 64 KiB", len(answer))
	}
}

func TestAsk_NeverRunsTheAnswer(t *testing.T) {
	loadConfig(t, "")
	hostile := "$(touch pwned) `touch pwned2`; touch pwned3"
	tmp := fakeAI(t, `printf '%s\n' '`+hostile+`'`)
	wd := t.TempDir()
	t.Chdir(wd)

	answer, err := Ask("", &bytes.Buffer{})
	if err != nil {
		t.Fatal(err)
	}
	if answer != hostile {
		t.Errorf("Ask() = %q, want it as text: %q", answer, hostile)
	}
	for _, dir := range []string{wd, tmp} {
		if left, _ := os.ReadDir(dir); len(left) != 0 {
			t.Errorf("%s holds %v after Ask", dir, left)
		}
	}
}

func TestAsk_Timeout(t *testing.T) {
	loadConfig(t, "ai:\n  timeout: 300ms\n")
	fakeAI(t, `sleep 30 & echo $! > "$PIDFILE"; wait`)
	pidFile := filepath.Join(t.TempDir(), "pid")
	t.Setenv("PIDFILE", pidFile)

	start := time.Now()
	_, err := Ask("", &bytes.Buffer{})
	if err == nil || !strings.Contains(err.Error(), "timeout") {
		t.Fatalf("Ask() error = %v, want one about the timeout", err)
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Errorf("Ask took %s to give up", elapsed)
	}

	data, err := os.ReadFile(pidFile)
	if err != nil {
		t.Fatal(err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		t.Fatal(err)
	}
	// Killed, it may linger a moment until its new parent reaps it.
	for deadline := time.Now().Add(5 * time.Second); syscall.Kill(pid, 0) == nil; {
		if time.Now().After(deadline) {
			t.Fatalf("sleep (pid %d) is still running", pid)
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestAsk_BadTimeout(t *testing.T) {
	loadConfig(t, "ai:\n  timeout: soon\n")
	fakeAI(t, `echo never`)

	if _, err := Ask("", &bytes.Buffer{}); err == nil || !strings.Contains(err.Error(), "ai.timeout") {
		t.Errorf("Ask() error = %v, want one naming ai.timeout", err)
	}
}

func TestAsk_MissingProgram(t *testing.T) {
	loadConfig(t, "")
	t.Setenv("PROJEKT_AI", "no-such-ai-program")

	if _, err := Ask("", &bytes.Buffer{}); err == nil || !strings.Contains(err.Error(), "no-such-ai-program") {
		t.Errorf("Ask() error = %v, want one naming the program", err)
	}
}

func TestAsk_FailingCommand(t *testing.T) {
	loadConfig(t, "")
	fakeAI(t, `echo boom >&2; exit 3`)

	var errOut bytes.Buffer
	if _, err := Ask("", &errOut); err == nil {
		t.Error("Ask() of a failing command returned no error")
	}
	if !strings.Contains(errOut.String(), "boom") {
		t.Errorf("errOut = %q, want the command's stderr", errOut.String())
	}
}
