package folderutil

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"sync"

	"gitlab.com/dynamo.foss/projekt/pkg/cli"
	"gitlab.com/dynamo.foss/projekt/pkg/lazypath"
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
}

// execResult is one folder's run, kept until it is its turn to be printed.
type execResult struct {
	folder ParsedFolder
	output string
	err    error
	// skipped says the folder was not there to run in.
	skipped bool
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

	folders, err := ParseConfig(lazypath.GetConfig())
	if err != nil {
		return err
	}
	folders = FilterByTags(folders, o.Tags)
	if len(folders) == 0 {
		cli.Warn("No folder matches")
		return nil
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
		if err := printExecResult(out, result, o); err != nil {
			return err
		}
	}

	if failed > 0 {
		return fmt.Errorf("%s failed in %d of %d folder(s)", o.Command[0], failed, len(results))
	}
	return nil
}

// runInFolders runs the command everywhere, with at most Forks at a time, and
// returns the results in the order the folders were given.
func runInFolders(folders []ParsedFolder, o ExecOptions) []execResult {
	results := make([]execResult, len(folders))
	semaphore := make(chan struct{}, execForks(o.Forks, len(folders)))

	var wg sync.WaitGroup
	for i, folder := range folders {
		wg.Add(1)

		go func(i int, folder ParsedFolder) {
			defer wg.Done()

			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			results[i] = runInFolder(folder, o)
		}(i, folder)
	}
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

// runInFolder runs the command in one folder.
func runInFolder(folder ParsedFolder, o ExecOptions) execResult {
	info, err := os.Stat(folder.Path)
	if err != nil || !info.IsDir() {
		// A folder that is not there is not a failure of the command: it is
		// something else to fix, and saying which is the useful part.
		return execResult{folder: folder, skipped: true}
	}

	cmd := exec.Command(o.Command[0], o.Command[1:]...)
	cmd.Dir = folder.Path

	// Both streams into one buffer, in the order the command wrote them, which
	// is how it would have looked on a terminal.
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output

	err = cmd.Run()
	return execResult{folder: folder, output: output.String(), err: err}
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
