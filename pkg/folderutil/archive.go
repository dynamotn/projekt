package folderutil

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"go.yaml.in/yaml/v3"

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

// RecordFile is where an archived folder remembers where it came from,
// relative to the folder.
const RecordFile = ".projekt/archived.yaml"

// ArchiveRecord is .projekt/archived.yaml inside an archived folder.
type ArchiveRecord struct {
	// Original is the absolute path the folder was moved from.
	Original   string    `yaml:"original"`
	ArchivedAt time.Time `yaml:"archived_at"`
	// Entry is the configuration entry that was dropped; nil for a child of
	// a workspace, which has none.
	Entry *lazypath.Folder `yaml:"entry,omitempty"`
}

// ArchivedFolder is one folder in the archive.
type ArchivedFolder struct {
	Name     string
	Path     string
	Original string    // "" when there is no record
	At       time.Time // zero when there is no record
}

// ListArchive returns what is in the archive folder o would use, by name.
func ListArchive(o ArchiveOptions) ([]ArchivedFolder, error) {
	root, err := archiveRoot(o)
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(root)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("cannot read %s: %w", root, err)
	}

	var result []ArchivedFolder
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		item := ArchivedFolder{Name: entry.Name(), Path: filepath.Join(root, entry.Name())}
		// A record that is missing, linked or unreadable is a folder without one.
		if projektFileIsPlain(item.Path, RecordFile, "an archive record") == nil {
			if data, err := os.ReadFile(filepath.Join(item.Path, RecordFile)); err == nil {
				var record ArchiveRecord
				if yaml.Unmarshal(data, &record) == nil {
					item.Original, item.At = record.Original, record.ArchivedAt
				}
			}
		}
		result = append(result, item)
	}
	return result, nil
}

// refuseBlockers fails when the project has working trees, or is declared in a
// file that is not this machine's own. Neither is overridden by --force: the
// first would leave trees pointing into a moved repository, the second would
// leave an entry that comes back on the next read.
func refuseBlockers(shortName string, folder ParsedFolder) error {
	key := lazypath.Canonical(folder.Path)
	for _, f := range lazypath.GetConfig().Folders {
		// Any entry for the path counts: one in the own file does not hide the
		// included one that would come back.
		if f.Source != "" && lazypath.Canonical(f.Path) == key {
			return fmt.Errorf("%s is declared in %s; archive it there", shortName, f.Source)
		}
	}

	var trees []string
	for _, w := range lazypath.GetConfig().Worktrees {
		if w.Project == folder.ShortName {
			trees = append(trees, w.Path)
		}
	}
	listing, err := gitOutput(folder.Path, "worktree", "list", "--porcelain")
	if err != nil {
		cli.Debug("Cannot list the working trees of %s: %v", folder.Path, err)
	} else if listed := listedWorktrees(listing); len(listed) > 1 {
		trees = append(trees, listed[1:]...)
	}
	if len(trees) == 0 {
		return nil
	}

	// A tree whose folder is gone is not removed, it is pruned.
	for _, tree := range trees {
		if _, err := os.Stat(tree); err != nil {
			return fmt.Errorf("%s has working trees, and %s is gone; run projekt worktree prune, then archive it",
				shortName, tree)
		}
	}
	return fmt.Errorf("%s has working trees; remove its working trees first (projekt worktree remove %s@…)",
		shortName, shortName)
}

// configuredEntry returns the configuration entry of a path, nil when it has
// none (a child of a workspace).
func configuredEntry(path string) *lazypath.Folder {
	key := lazypath.Canonical(path)
	for _, f := range lazypath.GetConfig().Folders {
		if lazypath.Canonical(f.Path) == key {
			return &f
		}
	}
	return nil
}

// writeRecord saves the record into the archived folder, never through a link.
func writeRecord(dir string, record ArchiveRecord) error {
	if err := projektFileIsPlain(dir, RecordFile, "an archive record"); err != nil {
		return err
	}
	data, err := yaml.Marshal(record)
	if err != nil {
		return err
	}
	path := filepath.Join(dir, RecordFile)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

// DefaultArchiveDir returns where finished projects go when nothing says
// otherwise: $XDG_DATA_HOME/projekt/archive.
func DefaultArchiveDir() string {
	data := lazypath.DataHome()
	if data == "" {
		return ""
	}
	return filepath.Join(data, "projekt", "archive")
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
		if dir = DefaultArchiveDir(); dir == "" {
			return "", errors.New("no home directory to keep the archive in: set HOME or PROJEKT_ARCHIVE_DIR")
		}
		return dir, nil
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

	entry := configuredEntry(folder.Path)
	if err := refuseBlockers(shortName, folder); err != nil {
		return err
	}
	// Refused now rather than after the move, when it could only be reported.
	if err := projektFileIsPlain(folder.Path, RecordFile, "an archive record"); err != nil {
		return err
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

	// The record goes in before the entry is dropped: if it cannot be written
	// the entry stays, and the user is told where both are.
	record := ArchiveRecord{Original: folder.Path, ArchivedAt: time.Now().UTC(), Entry: entry}
	if err := writeRecord(target, record); err != nil {
		return fmt.Errorf("moved %s to %s but could not write its archive record: %w; "+
			"the configuration entry still points at %s", folder.Path, target, err, folder.Path)
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
//
// Both callers make sure the target is not there yet, so a copy that fails
// half-way can remove what it wrote.
func moveFolder(from, to string) error {
	err := os.Rename(from, to)
	if err == nil {
		return nil
	}
	if !errors.Is(err, syscall.EXDEV) {
		// Anything else — no permission, a busy folder — a copy would hit as
		// well, or worse, half-way through removing the original.
		return fmt.Errorf("cannot move %s to %s: %w", from, to, err)
	}

	cli.Debug("Renaming %s failed, copying instead", from)
	// cp -a keeps the permissions, the timestamps and the symlinks, which a
	// hand-written copy gets wrong in a different way every time.
	cmd := cli.Command("cp", "-a", from+string(os.PathSeparator)+".", to)
	if err := os.MkdirAll(to, 0o755); err != nil {
		return fmt.Errorf("cannot create %s: %w", to, err)
	}
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output
	if err := cmd.Run(); err != nil {
		if cleanErr := os.RemoveAll(to); cleanErr != nil {
			cli.Warn("Cannot remove the partial copy %s: %v", to, cleanErr)
		}
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
	if !IsRepoRoot(path) {
		// Not a repository: there is no way to tell what is finished, so it is
		// not this command's place to guess.
		return ""
	}

	var reasons []string
	if output, err := gitOutput(path, "status", "--porcelain"); err == nil && strings.TrimSpace(output) != "" {
		reasons = append(reasons, "changes that are not committed")
	}
	if output, err := gitOutput(path, "log", "--oneline", "@{u}..HEAD"); err == nil && strings.TrimSpace(output) != "" {
		reasons = append(reasons, "commits that are not pushed")
	} else if err != nil {
		// No upstream at all: every commit is unpushed, and a repository that
		// was never pushed anywhere is exactly the one worth stopping for.
		if output, headErr := gitOutput(path, "log", "--oneline", "-1"); headErr == nil && strings.TrimSpace(output) != "" {
			if _, remoteErr := gitOutput(path, "rev-parse", "--abbrev-ref", "@{u}"); remoteErr != nil {
				reasons = append(reasons, "no upstream, so nothing has been pushed")
			}
		}
	}
	if output, err := gitOutput(path, "stash", "list"); err == nil && strings.TrimSpace(output) != "" {
		reasons = append(reasons, "something on the stash")
	}

	return strings.Join(reasons, " and ")
}
