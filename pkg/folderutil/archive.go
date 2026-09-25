package folderutil

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/OpenPeeDeeP/xdg"

	"gitlab.com/dynamo-tools/projekt/pkg/cli"
	"gitlab.com/dynamo-tools/projekt/pkg/lazypath"
)

// ArchiveDir is where finished projects go. It is bound to --archive-dir;
// when empty the XDG data home is used.
var ArchiveDir string

// ArchiveOptions drives `projekt folder archive`.
type ArchiveOptions struct {
	// To overrides where the project is moved to.
	To string
	// Force archives a project with work that is not committed or not pushed.
	Force bool
	// DryRun says what would happen without moving anything.
	DryRun bool
}

// DefaultArchiveDir returns where finished projects go when nothing says
// otherwise: $XDG_DATA_HOME/projekt/archive.
func DefaultArchiveDir() string {
	return filepath.Join(xdg.DataHome(), "projekt", "archive")
}

// archiveRoot works out the folder to archive into.
func archiveRoot(o ArchiveOptions) (string, error) {
	dir := strings.TrimSpace(o.To)
	if dir == "" {
		dir = strings.TrimSpace(ArchiveDir)
	}
	if dir == "" {
		dir = strings.TrimSpace(os.Getenv("PROJEKT_ARCHIVE_DIR"))
	}
	if dir == "" {
		return DefaultArchiveDir(), nil
	}
	return lazypath.NormalizePath(dir)
}

// ArchiveFolder moves a finished project out of the way and forgets about it.
//
// Nothing is deleted: the folder is moved, so the work is still there and
// `folder add` on the new path brings it back.
func ArchiveFolder(out io.Writer, shortName string, o ArchiveOptions) error {
	if err := lazypath.LoadError(); err != nil {
		return err
	}
	if _, _, isWorktree := lazypath.SplitWorktreeName(shortName); isWorktree {
		return fmt.Errorf("%s is a worktree; `projekt worktree remove` is what puts one away", shortName)
	}

	folder, err := FindFolder(shortName)
	if err != nil {
		return err
	}
	if info, statErr := os.Stat(folder.Path); statErr != nil || !info.IsDir() {
		return fmt.Errorf("%s is not there: %s", shortName, folder.Path)
	}

	root, err := archiveRoot(o)
	if err != nil {
		return err
	}
	target := filepath.Join(root, filepath.Base(filepath.Clean(folder.Path)))
	if _, err := os.Stat(target); err == nil {
		return fmt.Errorf("%s already exists; archive it under another name with --to", target)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("cannot access %s: %w", target, err)
	}

	if !o.Force {
		if reason := unfinished(folder.Path); reason != "" {
			return fmt.Errorf("%s has %s; finish it, or archive it anyway with --force", shortName, reason)
		}
	}

	if o.DryRun {
		_, err := fmt.Fprintf(out, "[DRY RUN] Would move %s to %s and drop it from the configuration\n",
			folder.Path, target)
		return err
	}

	if err := os.MkdirAll(root, 0o755); err != nil {
		return fmt.Errorf("cannot create %s: %w", root, err)
	}
	if err := moveFolder(folder.Path, target); err != nil {
		return err
	}

	// Only once the files are somewhere safe: an entry dropped for a move that
	// failed would lose the project twice over.
	if err := lazypath.RemoveFromConfig(folder.Path); err != nil {
		return fmt.Errorf("moved %s to %s but could not drop it from the configuration: %w",
			folder.Path, target, err)
	}

	_, err = fmt.Fprintf(out, "Archived %s to %s\n", shortName, target)
	return err
}

// moveFolder moves a folder, falling back to a copy when it would cross a
// filesystem, which a rename cannot do.
func moveFolder(from, to string) error {
	if err := os.Rename(from, to); err == nil {
		return nil
	}

	cli.Debug("Renaming %s failed, copying instead", from)
	// cp -a keeps the permissions, the timestamps and the symlinks, which a
	// hand-written copy gets wrong in a different way every time.
	cmd := exec.Command("cp", "-a", from+string(os.PathSeparator)+".", to)
	if err := os.MkdirAll(to, 0o755); err != nil {
		return fmt.Errorf("cannot create %s: %w", to, err)
	}
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("cannot copy %s to %s: %w: %s", from, to, err, strings.TrimSpace(output.String()))
	}
	if err := os.RemoveAll(from); err != nil {
		return fmt.Errorf("copied %s to %s but could not remove the original: %w", from, to, err)
	}
	return nil
}

// unfinished reports what is not safe to archive about a repository, or "".
//
// Archiving a project with work nobody else has is the one mistake this
// command can make that cannot be undone by moving the folder back.
func unfinished(path string) string {
	if !IsGitRepo(path) {
		// Not a repository: there is no way to tell what is finished, so it is
		// not this command's place to guess.
		return ""
	}

	var reasons []string
	if output, err := archiveGit(path, "status", "--porcelain"); err == nil && strings.TrimSpace(output) != "" {
		reasons = append(reasons, "changes that are not committed")
	}
	if output, err := archiveGit(path, "log", "--oneline", "@{u}..HEAD"); err == nil && strings.TrimSpace(output) != "" {
		reasons = append(reasons, "commits that are not pushed")
	} else if err != nil {
		// No upstream at all: every commit is unpushed, and a repository that
		// was never pushed anywhere is exactly the one worth stopping for.
		if output, headErr := archiveGit(path, "log", "--oneline", "-1"); headErr == nil && strings.TrimSpace(output) != "" {
			if _, remoteErr := archiveGit(path, "rev-parse", "--abbrev-ref", "@{u}"); remoteErr != nil {
				reasons = append(reasons, "no upstream, so nothing has been pushed")
			}
		}
	}
	if output, err := archiveGit(path, "stash", "list"); err == nil && strings.TrimSpace(output) != "" {
		reasons = append(reasons, "something on the stash")
	}

	return strings.Join(reasons, " and ")
}

// archiveGit runs a git command and returns what it wrote.
func archiveGit(path string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", path}, args...)...)

	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = io.Discard

	if err := cmd.Run(); err != nil {
		return "", err
	}
	return output.String(), nil
}
