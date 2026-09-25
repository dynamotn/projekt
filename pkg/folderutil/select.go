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

	"gitlab.com/dynamo-tools/projekt/pkg/cli"
	"gitlab.com/dynamo-tools/projekt/pkg/lazypath"
)

// DefaultPickers are the interactive filters to look for, in order. The first
// one on PATH is used; $PROJEKT_PICKER overrides the lot.
var DefaultPickers = []string{"fzf", "sk"}

// SelectOptions drives `projekt folder select`.
type SelectOptions struct {
	// Query narrows the list before anything is shown.
	Query string
	// Tags keeps only the projects carrying every one of these tags.
	Tags []string
	// With is the interactive filter to use, overriding what is found on PATH.
	With string
	// NoRecord leaves the jump history alone.
	NoRecord bool
	// In and Err are the terminal: the list is drawn on Err and the answer is
	// read from In, because the chosen path is what goes to the caller on Out.
	In  io.Reader
	Err io.Writer
}

// SelectFolder asks which project, and prints the one that is chosen.
//
// The path goes to out and nothing else does, so that `cd "$(projekt folder
// select)"` works: the question and the list are drawn on stderr.
func SelectFolder(out io.Writer, o SelectOptions) error {
	folders, err := ParseConfig(lazypath.GetConfig())
	if err != nil {
		return err
	}
	folders = FilterByTags(folders, o.Tags)

	matches := MatchFolders(folders, o.Query)
	switch len(matches) {
	case 0:
		if o.Query == "" {
			return fmt.Errorf("no project to choose from")
		}
		return fmt.Errorf("%w matching %q", ErrFolderNotFound, o.Query)
	case 1:
		// Nothing to ask: one candidate is an answer.
		return reportChoice(out, matches[0].Folder, o.NoRecord)
	}
	if matches[0].Exact() {
		// A whole name is an answer too, however many other projects contain it.
		return reportChoice(out, matches[0].Folder, o.NoRecord)
	}

	chosen, err := ask(matches, o)
	if err != nil {
		return err
	}
	return reportChoice(out, chosen, o.NoRecord)
}

// reportChoice prints the chosen path and remembers the jump.
func reportChoice(out io.Writer, folder ParsedFolder, noRecord bool) error {
	if _, err := fmt.Fprintln(out, folder.Path); err != nil {
		return err
	}
	if !noRecord {
		if err := lazypath.RecordVisit(folder.ShortName, folder.Path); err != nil {
			cli.Debug("Cannot record the jump: %v", err)
		}
	}
	return nil
}

// ask runs the interactive filter, or falls back to a numbered list.
func ask(matches []Match, o SelectOptions) (ParsedFolder, error) {
	if picker := pickerCommand(o.With); len(picker) > 0 {
		return askPicker(picker, matches, o)
	}
	return askNumbered(matches, o)
}

// pickerCommand returns the interactive filter to run, or nothing when there
// is none to run.
func pickerCommand(with string) []string {
	if trimmed := strings.TrimSpace(with); trimmed != "" {
		return strings.Fields(trimmed)
	}
	if fromEnv := strings.TrimSpace(os.Getenv("PROJEKT_PICKER")); fromEnv != "" {
		return strings.Fields(fromEnv)
	}
	for _, name := range DefaultPickers {
		if _, err := exec.LookPath(name); err == nil {
			return []string{name}
		}
	}
	return nil
}

// askPicker pipes the candidates through an interactive filter.
func askPicker(picker []string, matches []Match, o SelectOptions) (ParsedFolder, error) {
	// One short name per line, and nothing else: they are unique by
	// construction, so the filter needs no columns, and no flags of its own
	// have to be guessed at. Anything that reads lines and writes one back
	// works — fzf, sk, or a script of your own.
	byName := make(map[string]ParsedFolder, len(matches))
	var input bytes.Buffer
	for _, match := range matches {
		byName[match.Folder.ShortName] = match.Folder
		fmt.Fprintln(&input, match.Folder.ShortName)
	}

	cmd := exec.Command(picker[0], picker[1:]...)
	cmd.Stdin = &input
	// The filter draws on the terminal, which here is the error stream: the
	// output stream is carrying the answer back to the shell.
	cmd.Stderr = o.Err

	var chosen bytes.Buffer
	cmd.Stdout = &chosen

	if err := cmd.Run(); err != nil {
		// A filter exits non-zero when nothing was chosen, which is not a
		// failure: it is an answer of "never mind".
		return ParsedFolder{}, fmt.Errorf("nothing chosen")
	}

	folder, ok := byName[strings.TrimSpace(chosen.String())]
	if !ok {
		return ParsedFolder{}, fmt.Errorf("nothing chosen")
	}
	return folder, nil
}

// askNumbered prints the candidates and reads a number, for a machine with no
// interactive filter on it.
func askNumbered(matches []Match, o SelectOptions) (ParsedFolder, error) {
	in := o.In
	if in == nil {
		in = os.Stdin
	}
	errOut := o.Err
	if errOut == nil {
		errOut = os.Stderr
	}

	for i, match := range matches {
		fmt.Fprintf(errOut, "%3d  %-24s %s\n", i+1, match.Folder.ShortName, match.Folder.Path)
	}
	fmt.Fprintf(errOut, "Which one? [1-%d] ", len(matches))

	line, err := bufio.NewReader(in).ReadString('\n')
	if err != nil && strings.TrimSpace(line) == "" {
		fmt.Fprintln(errOut)
		return ParsedFolder{}, fmt.Errorf("nothing chosen")
	}

	choice, err := strconv.Atoi(strings.TrimSpace(line))
	if err != nil || choice < 1 || choice > len(matches) {
		return ParsedFolder{}, fmt.Errorf("%q is not one of 1 to %d", strings.TrimSpace(line), len(matches))
	}
	return matches[choice-1].Folder, nil
}
