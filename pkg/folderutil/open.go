package folderutil

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"gitlab.com/dynamo.foss/projekt/pkg/cli"
)

// OpenOptions drives `projekt folder open`.
type OpenOptions struct {
	// With overrides the editor, and may carry arguments: "code --new-window".
	With string
	// DryRun prints the command instead of running it.
	DryRun bool
	// In, Out and Err are the editor's streams. An editor is interactive, so
	// these are the real terminal in every case but a test.
	In       io.Reader
	Out, Err io.Writer
}

// OpenFolder opens a project in an editor.
//
// The editor runs with the project as its working directory as well as its
// argument, because half of them open the folder and the other half open
// whatever is in the current one.
func OpenFolder(out io.Writer, shortName string, o OpenOptions) error {
	folder, err := FindFolder(shortName)
	if err != nil {
		return err
	}
	if info, statErr := os.Stat(folder.Path); statErr != nil || !info.IsDir() {
		return fmt.Errorf("%s is not there: %s", shortName, folder.Path)
	}

	editor := cli.EditorCommand(os.Getenv)
	if trimmed := strings.TrimSpace(o.With); trimmed != "" {
		editor = strings.Fields(trimmed)
	}

	args := append(append([]string{}, editor[1:]...), folder.Path)
	if o.DryRun {
		_, err := fmt.Fprintf(out, "%s %s\n", editor[0], strings.Join(args, " "))
		return err
	}

	cli.Debug("Opening %s with %v", folder.Path, editor)

	cmd := exec.Command(editor[0], args...)
	cmd.Dir = folder.Path
	cmd.Stdin = o.In
	cmd.Stdout = o.Out
	cmd.Stderr = o.Err

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("editor %s failed: %w", editor[0], err)
	}
	return nil
}
