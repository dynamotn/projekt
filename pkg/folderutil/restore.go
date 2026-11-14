package folderutil

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"go.yaml.in/yaml/v3"

	"gitlab.com/dynamo-tools/projekt/pkg/lazypath"
)

// RestoreOptions drives `projekt folder restore`.
type RestoreOptions struct {
	// From is the archive folder; "" means where archive puts things.
	From string
	// To is the path to restore to; "" means the record's original.
	To string
	// DryRun says what would happen without changing anything.
	DryRun bool
}

// insideDir reports whether path is dir or lies below it.
func insideDir(path, dir string) bool {
	rel, err := filepath.Rel(lazypath.Canonical(dir), lazypath.Canonical(path))
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator))
}

// RestoreFolder moves an archived folder back, puts its configuration entry back
// (or registers it like `folder add` when there is no entry), and removes the record.
func RestoreFolder(out io.Writer, name string, o RestoreOptions) error {
	if err := lazypath.LoadError(); err != nil {
		return err
	}
	// The name is joined onto the archive folder, so it must stay one folder of it.
	if name == "" || name == "." || name == ".." || strings.ContainsAny(name, `/\`) {
		return fmt.Errorf("%q is not a folder of the archive: give its name, as --list shows it", name)
	}
	root, err := archiveRoot(ArchiveOptions{To: o.From})
	if err != nil {
		return err
	}
	source := filepath.Join(root, name)
	if info, err := os.Lstat(source); err != nil || !info.IsDir() {
		return fmt.Errorf("%s is not in the archive %s", name, root)
	}

	// Never read through a link: a record that points elsewhere could name any target.
	if err := projektFileIsPlain(source, RecordFile, "an archive record"); err != nil {
		return err
	}
	var record *ArchiveRecord
	if data, err := os.ReadFile(filepath.Join(source, RecordFile)); err == nil {
		record = &ArchiveRecord{}
		if err := yaml.Unmarshal(data, record); err != nil {
			return fmt.Errorf("the record of %s does not decode: %w; restore it with --to", name, err)
		}
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("cannot read the record of %s: %w", name, err)
	}

	target := strings.TrimSpace(o.To)
	if target != "" {
		if target, err = lazypath.NormalizePath(target); err != nil {
			return err
		}
	} else {
		switch {
		case record == nil:
			return fmt.Errorf("%s has no record of where it came from; say where with --to", name)
		case record.Original == "" || !filepath.IsAbs(record.Original) || insideDir(record.Original, root):
			return fmt.Errorf("the record of %s does not name a usable original path (%q); say where with --to",
				name, record.Original)
		}
		target = filepath.Clean(record.Original)
	}
	if insideDir(target, root) {
		return fmt.Errorf("%s is inside the archive; say another place with --to", target)
	}
	if _, err := os.Lstat(target); err == nil {
		return fmt.Errorf("%s already exists; restore under another path with --to", target)
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("cannot access %s: %w", target, err)
	}
	if configuredEntry(target) != nil {
		return fmt.Errorf("the configuration already has an entry for %s; remove it or restore with --to", target)
	}

	entry := &lazypath.Folder{}
	if record != nil && record.Entry != nil {
		*entry = *record.Entry
	}
	entry.Path = target

	if o.DryRun {
		_, err := fmt.Fprintf(out, "[DRY RUN] Would move %s to %s and add it to the configuration\n", source, target)
		return err
	}

	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return fmt.Errorf("cannot create %s: %w", filepath.Dir(target), err)
	}
	if err := moveFolder(source, target); err != nil {
		return err
	}
	if err := entry.AddToConfig(); err != nil {
		return fmt.Errorf("moved %s to %s but could not add it to the configuration: %w; "+
			"run `projekt folder add %s`, then delete %s", source, target, err, target, filepath.Join(target, RecordFile))
	}
	// A record left behind would make the project look unfinished to the next
	// archive. An emptied .projekt stays: it may have been there before.
	if record != nil {
		if err := os.Remove(filepath.Join(target, RecordFile)); err != nil {
			return fmt.Errorf("moved %s to %s and added it to the configuration but could not remove %s: %w; "+
				"delete it by hand", source, target, filepath.Join(target, RecordFile), err)
		}
	}

	_, err = fmt.Fprintf(out, "Restored %s to %s\n", name, target)
	return err
}
