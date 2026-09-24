package folderutil

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"gitlab.com/dynamo.foss/projekt/pkg/lazypath"
)

// ErrNotInAProject is returned when a path is not inside any known project.
var ErrNotInAProject = fmt.Errorf("not inside a project")

// CurrentOptions drives `projekt folder current`.
type CurrentOptions struct {
	// From is the folder to ask about. Empty means the current one.
	From string
	// PrintPath prints where the project is instead of what it is called.
	PrintPath bool
	// Quiet prints nothing and answers with the exit code alone.
	Quiet bool
}

// CurrentFolder says which project a path is in.
//
// The deepest match wins, so standing in a working tree gives the working
// tree, not the project it hangs off: both contain the path, and the closer
// one is the answer to "where am I".
func CurrentFolder(out io.Writer, o CurrentOptions) error {
	from := o.From
	if strings.TrimSpace(from) == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("cannot resolve the current folder: %w", err)
		}
		from = cwd
	}
	from, err := lazypath.NormalizePath(from)
	if err != nil {
		return err
	}
	// Follow symlinks: a project reached through one is still that project,
	// and the configuration holds the real path.
	if resolved, err := filepath.EvalSymlinks(from); err == nil {
		from = resolved
	}

	folders, err := ParseConfig(lazypath.GetConfig())
	if err != nil {
		return err
	}

	var best ParsedFolder
	var found bool
	for _, folder := range folders {
		if !contains(folder.Path, from) {
			continue
		}
		if !found || len(filepath.Clean(folder.Path)) > len(filepath.Clean(best.Path)) {
			best, found = folder, true
		}
	}
	if !found {
		return fmt.Errorf("%w: %s", ErrNotInAProject, from)
	}

	if o.Quiet {
		return nil
	}
	if o.PrintPath {
		_, err := fmt.Fprintln(out, best.Path)
		return err
	}
	_, err = fmt.Fprintln(out, best.ShortName)
	return err
}

// contains reports whether a path is the folder or lives under it.
func contains(folder, path string) bool {
	folder = filepath.Clean(folder)
	path = filepath.Clean(path)

	if folder == path {
		return true
	}
	// The separator matters: /home/me/app must not contain /home/me/app-docs.
	return strings.HasPrefix(path, folder+string(filepath.Separator))
}
