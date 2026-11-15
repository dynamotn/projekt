package folderutil

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"go.yaml.in/yaml/v3"

	"gitlab.com/dynamo-tools/projekt/pkg/cli"
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
	// Yes skips the question asked when the target comes from the record.
	Yes bool
	// In and Err are where that question is read and asked.
	In  io.Reader
	Err io.Writer
}

func hasControl(s string) bool {
	return strings.IndexFunc(s, unicode.IsControl) >= 0
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

	// What the record says is printed and written to the configuration: a
	// control character could redraw the terminal or hide in a field.
	if record != nil && record.Entry != nil {
		e := record.Entry
		for _, v := range append([]string{e.Name, e.Prefix, e.Description, e.RegexMatch}, e.Tags...) {
			if hasControl(v) {
				return fmt.Errorf("the record of %s has a control character in %q; edit or delete the record", name, v)
			}
		}
	}

	target := strings.TrimSpace(o.To)
	fromRecord := target == ""
	if !fromRecord {
		if target, err = lazypath.NormalizePath(target); err != nil {
			return err
		}
	} else {
		switch {
		case record == nil:
			return fmt.Errorf("%s has no record of where it came from; say where with --to", name)
		case hasControl(record.Original):
			return fmt.Errorf("the record of %s names the path %q, which has a control character; say where with --to",
				name, record.Original)
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

	// The record is data from a folder anybody may have edited: only the fields
	// that run nothing are put back, never hooks or a git section.
	entry := &lazypath.Folder{}
	var skipped []string
	if record != nil && record.Entry != nil {
		e := record.Entry
		*entry = lazypath.Folder{Name: e.Name, Prefix: e.Prefix, IsWorkspace: e.IsWorkspace,
			RegexMatch: e.RegexMatch, Priority: e.Priority, Tags: e.Tags, Description: e.Description}
		if !e.Hooks.IsEmpty() {
			skipped = append(skipped, "hooks enter: "+fmt.Sprintf("%q", e.Hooks.Enter)+
				", leave: "+fmt.Sprintf("%q", e.Hooks.Leave))
		}
		if e.Git != nil {
			git, _ := yaml.Marshal(e.Git)
			skipped = append(skipped, "git:\n"+strings.TrimRight(string(git), "\n"))
		}
	}
	entry.Path = target
	// A workspace child has no entry of its own: the move is all there is to do.
	_, covered := CoveringWorkspace(target)
	register := !(record != nil && record.Entry == nil && covered)
	say := func(format string, args ...any) error {
		_, err := fmt.Fprintf(out, format, args...)
		return err
	}
	notRestored := func() error {
		if len(skipped) == 0 {
			return nil
		}
		return say("The record also held the following, which is not restored; read it, then add what you "+
			"want back with `projekt config edit`:\n  %s\n", strings.Join(skipped, "\n  "))
	}

	if fromRecord {
		// Where the record says decides where the files land, so its parent must
		// already be there and the target is confirmed.
		if info, err := os.Stat(filepath.Dir(target)); err != nil || !info.IsDir() {
			return fmt.Errorf("%q does not exist; the record names %q: say where with --to", filepath.Dir(target), target)
		}
	}
	if o.DryRun {
		plan := "Would move %s to %q and add it to the configuration\n"
		if !register {
			plan = "Would move %s to %q; its workspace already lists it\n"
		}
		if err := say("[DRY RUN] "+plan, source, target); err != nil {
			return err
		}
		return notRestored()
	}
	if fromRecord && !o.Yes {
		if err := say("The record says %s came from %q\n", name, target); err != nil {
			return err
		}
		errOut := o.Err
		if errOut == nil {
			errOut = io.Discard
		}
		if ok, err := cli.Confirm(o.In, errOut, fmt.Sprintf("Restore it to %q? [y/N] ", target)); err != nil || !ok {
			return fmt.Errorf("nothing was restored; confirm %s, or use --yes, or restore elsewhere with --to", target)
		}
	}

	if !fromRecord {
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return fmt.Errorf("cannot create %s: %w", filepath.Dir(target), err)
		}
	}
	if err := moveFolder(source, target); err != nil {
		return err
	}
	if register {
		if err := entry.AddToConfig(); err != nil {
			// AddToConfig has already logged why.
			return fmt.Errorf("moved %s to %s but the configuration was not changed; the record at %s still holds "+
				"the entry: run `projekt folder add %s` and set it again, then delete the record",
				source, target, filepath.Join(target, RecordFile), target)
		}
	}
	// A record left behind would make the project look unfinished to the next
	// archive. An emptied .projekt stays: it may have been there before.
	if record != nil {
		if err := os.Remove(filepath.Join(target, RecordFile)); err != nil {
			return fmt.Errorf("moved %s to %s and added it to the configuration but could not remove %s: %w; "+
				"delete it by hand", source, target, filepath.Join(target, RecordFile), err)
		}
	}

	if err := say("Restored %s to %s\n", name, target); err != nil {
		return err
	}
	return notRestored()
}
