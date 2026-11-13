// Package aiutil hands a prompt to the owner's own AI command-line tool and
// reads its answer back as text.
//
// projekt holds no key and speaks to no vendor: the command is whatever the
// owner configured, run without a shell, and its answer is only ever text to
// show, never something to run.
package aiutil

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unicode"

	"gitlab.com/dynamo-tools/projekt/pkg/cli"
	"gitlab.com/dynamo-tools/projekt/pkg/lazypath"
)

const (
	// defaultTimeout is how long an answer may take when ai.timeout is unset.
	defaultTimeout = 2 * time.Minute
	// maxAnswer is more than any sensible answer, and caps what a runaway
	// command can make projekt hold.
	maxAnswer = 64 << 10
)

// ErrNotConfigured is returned when neither PROJEKT_AI nor ai.command is set.
var ErrNotConfigured = errors.New(`no AI command: set ai.command in the configuration, for example "claude -p", or PROJEKT_AI`)

// Command returns the AI command line to run, split into words, from
// PROJEKT_AI or the owner's own ai.command.
func Command() ([]string, error) {
	line := os.Getenv("PROJEKT_AI")
	if strings.TrimSpace(line) == "" {
		// The owner's file only: an included file must not choose what runs.
		line = lazypath.OwnConfig().AI.Command
	}
	words := cli.SplitShellWords(line)
	if len(words) == 0 {
		return nil, ErrNotConfigured
	}
	return words, nil
}

// Ask runs the AI command with prompt on stdin in an empty temporary folder,
// stops it after the configured timeout, and returns its cleaned answer.
// errOut receives the command's stderr.
func Ask(prompt string, errOut io.Writer) (string, error) {
	words, err := Command()
	if err != nil {
		return "", err
	}
	timeout, err := configuredTimeout()
	if err != nil {
		return "", err
	}

	program, err := exec.LookPath(words[0])
	if err == nil {
		// It runs in another folder, where a relative path means nothing.
		program, err = filepath.Abs(program)
	}
	if err != nil {
		return "", fmt.Errorf("cannot run the AI command %s: %w", words[0], err)
	}

	// An empty folder, so an agent CLI does not find itself inside a project.
	dir, err := os.MkdirTemp("", "projekt-ai-")
	if err != nil {
		return "", err
	}
	defer func() { _ = os.RemoveAll(dir) }()

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	var answer capped
	// The owner's program, named by their own configuration or environment,
	// started directly with argv: no shell ever sees the line or the answer.
	cmd := exec.CommandContext(ctx, program, words[1:]...) // nosemgrep: go.lang.security.audit.dangerous-exec-command.dangerous-exec-command
	cmd.Dir = dir
	cmd.Stdin = strings.NewReader(prompt)
	cmd.Stdout = &answer
	cmd.Stderr = errOut
	// Its own process group, killed whole: a script's children go with it.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
	// A child that keeps the pipes open must not hold Wait past the timeout.
	cmd.WaitDelay = time.Second

	err = cmd.Run()
	if ctx.Err() == context.DeadlineExceeded {
		return "", fmt.Errorf("the AI command %s gave no answer within the timeout of %s (ai.timeout)", words[0], timeout)
	}
	if err != nil {
		return "", fmt.Errorf("the AI command %s failed: %w", words[0], err)
	}
	return clean(answer.String()), nil
}

// configuredTimeout reads ai.timeout from the owner's file.
func configuredTimeout() (time.Duration, error) {
	value := strings.TrimSpace(lazypath.OwnConfig().AI.Timeout)
	if value == "" {
		return defaultTimeout, nil
	}
	timeout, err := time.ParseDuration(value)
	if err != nil || timeout <= 0 {
		return 0, fmt.Errorf("ai.timeout %q is not a positive duration such as 2m or 90s", value)
	}
	return timeout, nil
}

// clean makes an answer plain text: valid UTF-8, no control characters but
// newline and tab, no surrounding blanks.
func clean(answer string) string {
	answer = strings.Map(func(r rune) rune {
		if r != '\n' && r != '\t' && unicode.IsControl(r) {
			return -1
		}
		return r
	}, strings.ToValidUTF8(answer, ""))
	return strings.TrimSpace(answer)
}

// capped keeps the first maxAnswer bytes written to it and drops the rest,
// still reporting them written so the command is never blocked on its pipe.
type capped struct{ strings.Builder }

func (c *capped) Write(p []byte) (int, error) {
	if room := maxAnswer - c.Len(); room > 0 {
		c.Builder.Write(p[:min(len(p), room)])
	}
	return len(p), nil
}
