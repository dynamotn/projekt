package folderutil

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"

	"gitlab.com/dynamo.foss/projekt/pkg/cli"
	"gitlab.com/dynamo.foss/projekt/pkg/lazypath"
)

// StatusOptions drives `projekt folder status`.
type StatusOptions struct {
	// Tags keeps only the folders carrying every one of these tags.
	Tags []string
	// Forks is how many folders are read at once.
	Forks int
	// DirtyOnly keeps only the folders with something to attend to: changes,
	// commits not pushed, commits not merged, or a stash.
	DirtyOnly bool
	NamesOnly bool
	cli.ListOutputOption
}

// FolderStatus is what one project folder looks like right now.
type FolderStatus struct {
	Name string
	Path string
	// State is why the rest is empty, when it is: missing, or not a repo.
	State string
	// Branch is the checked out branch, or a short commit for a detached head.
	Branch string
	// Changed counts the files git would report, tracked and untracked alike.
	Changed int
	// Ahead and Behind compare the branch with its upstream.
	Ahead, Behind int
	// Stashes counts the entries on the stash.
	Stashes int
	// Last is how long ago the last commit was, in git's own words.
	Last string
}

// The states a folder can be in when there is nothing to report about it.
const (
	statusOK      = "ok"
	statusMissing = "missing"
	statusNoRepo  = "not a repo"
	statusNoHead  = "no commit yet"
)

// NeedsAttention reports whether anything about the folder is worth a look.
func (s FolderStatus) NeedsAttention() bool {
	if s.State != statusOK {
		return true
	}
	return s.Changed > 0 || s.Ahead > 0 || s.Behind > 0 || s.Stashes > 0
}

// Summary is the short answer for a folder, for the text formats.
func (s FolderStatus) Summary() string {
	if s.State != statusOK {
		return s.State
	}

	var parts []string
	if s.Changed > 0 {
		parts = append(parts, fmt.Sprintf("%d changed", s.Changed))
	}
	if s.Ahead > 0 {
		parts = append(parts, fmt.Sprintf("%d ahead", s.Ahead))
	}
	if s.Behind > 0 {
		parts = append(parts, fmt.Sprintf("%d behind", s.Behind))
	}
	if s.Stashes > 0 {
		parts = append(parts, fmt.Sprintf("%d stashed", s.Stashes))
	}
	if len(parts) == 0 {
		return "clean"
	}
	return strings.Join(parts, ", ")
}

// StatusFolders reports on every selected project folder.
func StatusFolders(out io.Writer, o *StatusOptions) error {
	folders, err := ParseConfig(lazypath.GetConfig())
	if err != nil {
		return err
	}
	folders = FilterByTags(folders, o.Tags)

	statuses := readStatuses(folders, o.Forks)
	if o.DirtyOnly {
		kept := statuses[:0]
		for _, status := range statuses {
			if status.NeedsAttention() {
				kept = append(kept, status)
			}
		}
		statuses = kept
	}

	if len(statuses) == 0 && o.Output != cli.OutputJSON {
		if o.DirtyOnly {
			cli.Info("Nothing to attend to")
		} else {
			cli.Warn("No folder matches")
		}
		return nil
	}

	view := cli.ListView{Columns: []cli.ListColumn{
		{Header: "NAME", Key: "name"},
		{Header: "BRANCH", Key: "branch"},
		{Header: "STATE", Key: "state"},
		{Header: "CHANGED", Key: "changed"},
		{Header: "AHEAD", Key: "ahead"},
		{Header: "BEHIND", Key: "behind"},
		{Header: "STASH", Key: "stashes"},
		{Header: "LAST COMMIT", Key: "last"},
	}}
	if o.NamesOnly {
		view.Columns = view.Columns[:1]
	}

	for _, status := range statuses {
		if o.NamesOnly {
			view.AppendRow(status.Name)
			continue
		}
		view.AppendRow(status.Name, status.Branch, status.State,
			status.Changed, status.Ahead, status.Behind, status.Stashes, status.Last)
	}

	return cli.EncodeList(out, view, o.ListOutputOption)
}

// readStatuses reads every folder, several at a time, and keeps the order.
func readStatuses(folders []ParsedFolder, forks int) []FolderStatus {
	statuses := make([]FolderStatus, len(folders))
	if len(folders) == 0 {
		return statuses
	}
	semaphore := make(chan struct{}, statusForks(forks, len(folders)))

	var wg sync.WaitGroup
	for i, folder := range folders {
		wg.Add(1)

		go func(i int, folder ParsedFolder) {
			defer wg.Done()

			semaphore <- struct{}{}
			defer func() { <-semaphore }()

			statuses[i] = readStatus(folder)
		}(i, folder)
	}
	wg.Wait()

	return statuses
}

func statusForks(forks, folderCount int) int {
	if forks <= 0 {
		forks = DefaultSyncForks
	}
	if forks > folderCount {
		forks = folderCount
	}
	return forks
}

// readStatus reads one folder.
func readStatus(folder ParsedFolder) FolderStatus {
	status := FolderStatus{Name: folder.ShortName, Path: folder.Path, State: statusOK}

	if info, err := os.Stat(folder.Path); err != nil || !info.IsDir() {
		status.State = statusMissing
		return status
	}
	if !IsGitRepo(folder.Path) {
		status.State = statusNoRepo
		return status
	}

	// One call gives the branch, the comparison with upstream and every file
	// git would mention; the other two are cheap and answer the rest.
	porcelain, err := gitOutput(folder.Path, "status", "--porcelain=v2", "--branch", "--untracked-files=normal")
	if err != nil {
		status.State = statusNoRepo
		return status
	}
	parseStatusPorcelain(&status, porcelain)

	if last, err := gitOutput(folder.Path, "log", "-1", "--format=%cr"); err == nil {
		status.Last = strings.TrimSpace(last)
	} else {
		// A repository with no commit has no branch to compare and no date.
		status.State = statusNoHead
	}
	if stashes, err := gitOutput(folder.Path, "stash", "list"); err == nil {
		status.Stashes = len(outputLines(stashes))
	}

	return status
}

// parseStatusPorcelain reads `git status --porcelain=v2 --branch`.
func parseStatusPorcelain(status *FolderStatus, output string) {
	scanner := bufio.NewScanner(strings.NewReader(output))
	for scanner.Scan() {
		line := scanner.Text()
		switch {
		case strings.HasPrefix(line, "# branch.head "):
			status.Branch = strings.TrimPrefix(line, "# branch.head ")
		case strings.HasPrefix(line, "# branch.ab "):
			status.Ahead, status.Behind = parseAheadBehind(strings.TrimPrefix(line, "# branch.ab "))
		case strings.HasPrefix(line, "#"):
			// Another header, of which only those two matter here.
		case line != "":
			// 1, 2, u and ? lines are all a file with something about it.
			status.Changed++
		}
	}

	if status.Branch == "(detached)" {
		status.Branch = "detached"
	}
}

// parseAheadBehind reads the "+1 -2" of a branch.ab header.
func parseAheadBehind(value string) (ahead, behind int) {
	for _, field := range strings.Fields(value) {
		if len(field) < 2 {
			continue
		}
		number, err := strconv.Atoi(field[1:])
		if err != nil {
			continue
		}
		switch field[0] {
		case '+':
			ahead = number
		case '-':
			behind = number
		}
	}
	return ahead, behind
}

// gitOutput runs a git command and returns what it wrote.
func gitOutput(repoPath string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", repoPath}, args...)...)

	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = io.Discard

	if err := cmd.Run(); err != nil {
		return "", err
	}
	return output.String(), nil
}

// outputLines splits output into its non-empty lines.
func outputLines(output string) []string {
	trimmed := strings.TrimRight(output, "\n")
	if trimmed == "" {
		return nil
	}
	return strings.Split(trimmed, "\n")
}
