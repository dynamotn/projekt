package folderutil

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"os"
	"os/exec"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"gitlab.com/dynamo-tools/projekt/pkg/cli"
	"gitlab.com/dynamo-tools/projekt/pkg/lazypath"
)

// ExecOptions drives `projekt folder exec`.
type ExecOptions struct {
	// Command is the command and its arguments, run in each project folder.
	Command []string
	// Tags keeps only the folders carrying every one of these tags.
	Tags []string
	// Forks is how many folders are worked on at once.
	Forks int
	// DryRun lists what would be run, without running any of it.
	DryRun bool
	// Quiet reports only the folders where the command failed, which is what
	// makes `exec` usable as a question: which of my projects is not clean?
	Quiet bool
	// NoOutput drops the command's own output, keeping the verdict per folder.
	NoOutput bool
	// Output, when set, prints one JSON object per folder instead of the text
	// report. Only cli.OutputJSON is accepted.
	Output cli.OutputFormat
	// Timeout stops the command in a folder after this long; zero means no
	// limit.
	Timeout time.Duration
	// FailFast starts no new folder after the first failure. The folders
	// already running finish.
	FailFast bool
}

// execResult is one folder's run, kept until it is its turn to be printed.
type execResult struct {
	folder ParsedFolder
	output string
	err    error
	// skipped says the folder was not there to run in.
	skipped bool
	// notStarted says the run stopped, at a failure or an interrupt, before
	// this folder's turn.
	notStarted bool
	// took is how long the command ran.
	took time.Duration
}

// execRecord is one folder's run as `-o json` prints it.
type execRecord struct {
	Name    string  `json:"name"`
	Path    string  `json:"path"`
	Exit    int     `json:"exit"`
	Output  string  `json:"output"`
	Seconds float64 `json:"seconds"`
}

// ExecInFolders runs one command in every selected project folder.
//
// The folders are worked on concurrently but reported in configuration order,
// so that the output can be read, diffed and grepped: a listing that comes out
// in a different order every time is not one you can compare.
func ExecInFolders(out io.Writer, o ExecOptions) error {
	if len(o.Command) == 0 {
		return fmt.Errorf("no command given")
	}
	if o.Output != "" {
		if o.Output != cli.OutputJSON {
			return fmt.Errorf("folder exec prints only json with --output, not %q", o.Output)
		}
		if o.Quiet || o.NoOutput {
			return fmt.Errorf("--quiet and --no-output shape the text report: drop them with -o json")
		}
	}

	folders, err := ParseConfig(lazypath.GetConfig())
	if err != nil {
		return err
	}
	folders = FilterByTags(folders, o.Tags)
	if len(folders) == 0 {
		if o.Output == cli.OutputJSON {
			_, err := fmt.Fprintln(out, "[]")
			return err
		}
		cli.Warn("No folder matches")
		return nil
	}

	if o.DryRun && o.Output == cli.OutputJSON {
		// Nothing but the array on stdout, so a script can still parse it.
		results := make([]execResult, len(folders))
		for i, folder := range folders {
			results[i] = execResult{folder: folder}
		}
		return printExecJSON(out, results)
	}
	if o.DryRun {
		for _, folder := range folders {
			if _, err := fmt.Fprintf(out, "[DRY RUN] %s: %s\n", folder.ShortName, strings.Join(o.Command, " ")); err != nil {
				return err
			}
		}
		return nil
	}

	results := runInFolders(folders, o)

	var failed int
	for _, result := range results {
		if result.err != nil {
			failed++
		}
	}
	if o.Output == cli.OutputJSON {
		if err := printExecJSON(out, results); err != nil {
			return err
		}
	} else {
		for _, result := range results {
			if err := printExecResult(out, result, o); err != nil {
				return err
			}
		}
	}

	if failed > 0 {
		return fmt.Errorf("%s failed in %d of %d folder(s)", o.Command[0], failed, len(results))
	}
	return nil
}

// runInFolders runs the command everywhere, with at most Forks at a time, and
// returns the results in the order the folders were given. The folders start
// in that order too, so that --fail-fast stops at a predictable place.
func runInFolders(folders []ParsedFolder, o ExecOptions) []execResult {
	results := make([]execResult, len(folders))

	ctx := context.Background()
	if o.Timeout > 0 {
		// Only a command with a timeout runs in a group of its own, out of
		// reach of the terminal's Ctrl-C.
		var stop context.CancelFunc
		ctx, stop = interruptible()
		defer stop()
	}

	var failed atomic.Bool
	next := make(chan int)
	var wg sync.WaitGroup
	for range execForks(o.Forks, len(folders)) {
		wg.Go(func() {
			for i := range next {
				if ctx.Err() != nil || o.FailFast && failed.Load() {
					results[i] = execResult{folder: folders[i], notStarted: true}
					continue
				}
				results[i] = runInFolder(ctx, folders[i], o)
				if results[i].err != nil {
					failed.Store(true)
				}
			}
		})
	}
	for i := range folders {
		next <- i
	}
	close(next)
	wg.Wait()

	return results
}

// execForks is how many folders to work on at once. More workers than folders
// only adds idle goroutines.
func execForks(forks, folderCount int) int {
	if forks <= 0 {
		forks = DefaultSyncForks
	}
	if forks > folderCount {
		forks = folderCount
	}
	return forks
}

// runInFolder runs the command in one folder, until its timeout.
func runInFolder(ctx context.Context, folder ParsedFolder, o ExecOptions) execResult {
	info, err := os.Stat(folder.Path)
	if err != nil || !info.IsDir() {
		// A folder that is not there is not a failure of the command: it is
		// something else to fix, and saying which is the useful part.
		return execResult{folder: folder, skipped: true}
	}

	cmd := exec.Command(o.Command[0], o.Command[1:]...)
	if o.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, o.Timeout)
		defer cancel()
		cmd = exec.CommandContext(ctx, o.Command[0], o.Command[1:]...)
		cli.OwnGroup(cmd)
	}
	cmd.Dir = folder.Path

	// Both streams into one buffer, in the order the command wrote them, which
	// is how it would have looked on a terminal.
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output

	start := time.Now()
	err = cmd.Run()
	if err != nil && ctx.Err() != nil {
		if err = stopped(ctx, err); errors.Is(err, errTimedOut) {
			err = fmt.Errorf("%w after %s", err, o.Timeout)
		}
	}
	return execResult{folder: folder, output: output.String(), err: err, took: time.Since(start)}
}

// printExecJSON writes every folder's run as one array, in the order given. A
// folder that is not on disk is exit -1 with "missing" as its output.
func printExecJSON(out io.Writer, results []execResult) error {
	records := make([]execRecord, len(results))
	for i, r := range results {
		rec := execRecord{
			Name:    r.folder.ShortName,
			Path:    r.folder.Path,
			Output:  r.output,
			Seconds: math.Round(r.took.Seconds()*100) / 100,
		}
		var exitErr *exec.ExitError
		switch {
		case r.skipped:
			rec.Exit, rec.Output = -1, "missing"
		case r.notStarted:
			rec.Exit, rec.Output = -1, "not started"
		case errors.Is(r.err, errTimedOut) || errors.Is(r.err, errInterrupted):
			// What it printed before it was stopped, then why it stopped.
			if rec.Output != "" && !strings.HasSuffix(rec.Output, "\n") {
				rec.Output += "\n"
			}
			rec.Exit, rec.Output = -1, rec.Output+r.err.Error()+"\n"
		case errors.As(r.err, &exitErr):
			rec.Exit = exitErr.ExitCode()
		case r.err != nil:
			// The command never started: the start error is all there is to say.
			rec.Exit, rec.Output = -1, r.err.Error()
		}
		records[i] = rec
	}

	enc := json.NewEncoder(out)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	return enc.Encode(records)
}

// printExecResult writes one folder's run.
func printExecResult(out io.Writer, result execResult, o ExecOptions) error {
	if result.skipped {
		if o.Quiet {
			return nil
		}
		_, err := fmt.Fprintf(out, "%s: missing %s\n", result.folder.ShortName, result.folder.Path)
		return err
	}
	if result.notStarted {
		_, err := fmt.Fprintf(out, "%s: not started\n", result.folder.ShortName)
		return err
	}

	// In quiet mode only a failure is worth a line, which turns `exec` into a
	// filter: the folders that answer yes.
	if o.Quiet && result.err == nil {
		return nil
	}

	status := "ok"
	if result.err != nil {
		status = execFailure(result.err)
	}
	if _, err := fmt.Fprintf(out, "%s: %s\n", result.folder.ShortName, status); err != nil {
		return err
	}

	if o.NoOutput {
		return nil
	}
	for _, line := range outputLines(result.output) {
		if _, err := fmt.Fprintf(out, "  %s\n", line); err != nil {
			return err
		}
	}
	return nil
}

// execFailure turns a run error into something worth reading.
func execFailure(err error) string {
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return fmt.Sprintf("exit %d", exitErr.ExitCode())
	}
	return err.Error()
}
