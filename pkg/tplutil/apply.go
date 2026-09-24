package tplutil

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// ChangeStatus is what applying a template would do to one file.
type ChangeStatus string

const (
	// ChangeAdded is a file the template writes and the project does not have.
	ChangeAdded ChangeStatus = "added"
	// ChangeUpdated is a file the template writes differently, and which has
	// not been touched since it was rendered, so replacing it loses nothing.
	ChangeUpdated ChangeStatus = "updated"
	// ChangeUnchanged is a file that is already what the template says.
	ChangeUnchanged ChangeStatus = "unchanged"
	// ChangeConflict is a file the template writes differently *and* which was
	// edited by hand since. Replacing it would throw that away, so it takes
	// --force.
	ChangeConflict ChangeStatus = "conflict"
	// ChangeRemoved is a file the template used to write and no longer does.
	// It is deleted only with --prune.
	ChangeRemoved ChangeStatus = "removed"
)

// Change is one file's part of an apply.
type Change struct {
	// Path is relative to the project, with "/" whatever the platform is.
	Path string
	// Status is what would happen to it.
	Status ChangeStatus
	// Before is what is on disk, After what the template renders.
	Before, After []byte
	// Attrs are what the template's name asked the file to be.
	Attrs Attributes
	// recorded is the hash the project remembers writing, which is what a
	// change the apply leaves alone must go on remembering.
	recorded string
}

// Writes reports whether this change touches the disk at all.
func (c Change) Writes() bool {
	return c.Status != ChangeUnchanged
}

// ApplyOptions drives `t diff` and `t apply`.
type ApplyOptions struct {
	// Template is the folder template to apply.
	Template Template
	// Dest is the project it was rendered into.
	Dest string
	// Name overrides the recorded `.Name`.
	Name string
	// Values are given on top of the recorded ones, which are replayed.
	Values Values
	// Force rewrites the files that were edited by hand since.
	Force bool
	// Prune deletes the files the template no longer writes.
	Prune bool
	// DryRun works the whole thing out without writing: `t diff`.
	DryRun bool
	// In, Prompt and Interactive are what a prompt function renders with.
	In          io.Reader
	Prompt      io.Writer
	Interactive bool
}

// PlanApply works out what applying a template to a project would do.
//
// The project's record is what makes the difference between "this file is out
// of date" and "somebody edited this file": a file whose hash still matches
// what was written is ours to replace, anything else is theirs.
func PlanApply(o ApplyOptions) ([]Change, RenderRecord, error) {
	if !o.Template.IsDir() {
		return nil, RenderRecord{}, fmt.Errorf("%s is a file template; only a folder template can be applied", o.Template.Name)
	}
	root, err := filepath.Abs(dirOrCurrent(o.Dest))
	if err != nil {
		return nil, RenderRecord{}, fmt.Errorf("cannot resolve %s: %w", o.Dest, err)
	}

	record, err := LoadRecord(root)
	if err != nil {
		return nil, RenderRecord{}, err
	}
	previous, remembered := record.Find(o.Template.Name)

	// The recorded values are replayed, and anything given now wins over them,
	// so `t apply go-cli . --set ci=true` changes one answer and keeps the
	// rest.
	render := RenderOptions{
		Template:    o.Template,
		Dest:        root,
		Name:        firstNonEmpty(o.Name, previous.Name),
		Values:      MergeValues(previous.Values, o.Values),
		In:          o.In,
		Prompt:      o.Prompt,
		Interactive: o.Interactive,
	}
	values, err := WithData(render)
	if err != nil {
		return nil, RenderRecord{}, err
	}
	render.Values = values

	files, err := Collect(render)
	if err != nil {
		return nil, RenderRecord{}, err
	}

	next := RenderRecord{
		Template:   o.Template.Name,
		Name:       nameOf(render),
		RenderedAt: time.Now().UTC(),
		Values:     render.Values,
		Files:      map[string]string{},
	}

	var changes []Change
	rendered := map[string]bool{}
	for _, file := range files {
		if file.IsDir {
			continue
		}
		rendered[file.Rel] = true
		next.Files[file.Rel] = hashOf(file.Content)

		before, err := currentOf(filepath.Join(root, filepath.FromSlash(file.Rel)), file.Attrs)
		if err != nil {
			return nil, RenderRecord{}, err
		}
		changes = append(changes, Change{
			Path:     file.Rel,
			Status:   statusOf(before, file.Content, previous.Files[file.Rel], remembered),
			Before:   before,
			After:    file.Content,
			Attrs:    file.Attrs,
			recorded: previous.Files[file.Rel],
		})
	}

	// What the template used to write and no longer does: an `.ignore` that
	// now covers it, or a file taken out of the template.
	for path, hash := range previous.Files {
		if rendered[path] {
			continue
		}
		target := filepath.Join(root, filepath.FromSlash(path))
		before, err := currentOf(target, Attributes{})
		if err != nil {
			return nil, RenderRecord{}, err
		}
		if before == nil {
			// Already gone; only the record still remembers it.
			continue
		}
		status := ChangeRemoved
		if hashOf(before) != hash {
			// Edited since, so deleting it would throw work away.
			status = ChangeConflict
		}
		changes = append(changes, Change{Path: path, Status: status, Before: before, recorded: hash})
	}

	sort.Slice(changes, func(i, j int) bool { return changes[i].Path < changes[j].Path })
	return changes, next, nil
}

// statusOf decides what one file's change is.
func statusOf(before, after []byte, recorded string, remembered bool) ChangeStatus {
	switch {
	case before == nil:
		return ChangeAdded
	case string(before) == string(after):
		return ChangeUnchanged
	case remembered && hashOf(before) == recorded:
		// Exactly what was written last time, so it is ours to replace.
		return ChangeUpdated
	default:
		return ChangeConflict
	}
}

// Apply renders a template over a project it was already rendered into.
//
// Nothing that was edited by hand is touched without --force, and nothing is
// deleted without --prune: an apply that quietly threw work away would only be
// run once.
func Apply(o ApplyOptions) ([]Change, error) {
	changes, next, err := PlanApply(o)
	if err != nil {
		return nil, err
	}
	if o.DryRun {
		return changes, nil
	}

	root, err := filepath.Abs(dirOrCurrent(o.Dest))
	if err != nil {
		return nil, err
	}

	for _, change := range changes {
		target := filepath.Join(root, filepath.FromSlash(change.Path))
		switch change.Status {
		case ChangeUnchanged:
			continue
		case ChangeAdded, ChangeUpdated:
			if err := writeFile(target, change.After, true, change.Attrs); err != nil {
				return nil, err
			}
		case ChangeConflict:
			if !o.Force {
				// It stays as it is, and so does the record's memory of it:
				// pretending it was rewritten would make the next apply lie.
				remember(next, change)
				continue
			}
			if change.After == nil {
				if err := remove(root, target); err != nil {
					return nil, err
				}
				continue
			}
			if err := writeFile(target, change.After, true, change.Attrs); err != nil {
				return nil, err
			}
		case ChangeRemoved:
			if !o.Prune {
				// Still worth remembering: a file forgotten here could never
				// be pruned later, because nothing would know it came from
				// the template.
				remember(next, change)
				continue
			}
			if err := remove(root, target); err != nil {
				return nil, err
			}
		}
	}

	if err := recordRender(root, next); err != nil {
		return nil, err
	}
	return changes, nil
}

// remember keeps a change the apply left alone in the record, exactly as it
// was last written, so the next apply reaches the same conclusion.
func remember(record RenderRecord, change Change) {
	if change.recorded == "" {
		delete(record.Files, change.Path)
		return
	}
	record.Files[change.Path] = change.recorded
}

// remove deletes a file the template no longer writes, and the folders above
// it that held nothing else.
//
// It stops at the project: emptying a project is not the same as removing a
// file from it, and `t apply --prune` was never asked to do the first.
func remove(root, target string) error {
	if err := os.Remove(target); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("cannot remove %s: %w", target, err)
	}
	// An empty folder left behind is litter; a folder with anything else in it
	// is somebody's, and Remove refuses it of its own accord.
	for dir := filepath.Dir(target); dir != root && strings.HasPrefix(dir, root+string(filepath.Separator)); dir = filepath.Dir(dir) {
		if err := os.Remove(dir); err != nil {
			return nil
		}
	}
	return nil
}

// currentOf reads what is at a path, returning nil when there is nothing.
//
// A symbolic link is what it points at, which is what the template rendered.
func currentOf(target string, attrs Attributes) ([]byte, error) {
	if attrs.Symlink {
		link, err := os.Readlink(target)
		if err != nil {
			if os.IsNotExist(err) {
				return nil, nil
			}
			return nil, fmt.Errorf("cannot read %s: %w", target, err)
		}
		return []byte(link + "\n"), nil
	}

	data, err := os.ReadFile(target)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("cannot read %s: %w", target, err)
	}
	return data, nil
}

// Summary counts the changes by status, for the line a command ends on.
func Summary(changes []Change) string {
	counts := map[ChangeStatus]int{}
	for _, change := range changes {
		counts[change.Status]++
	}

	var parts []string
	for _, status := range []ChangeStatus{ChangeAdded, ChangeUpdated, ChangeConflict, ChangeRemoved, ChangeUnchanged} {
		if counts[status] > 0 {
			parts = append(parts, fmt.Sprintf("%d %s", counts[status], status))
		}
	}
	if len(parts) == 0 {
		return "nothing to do"
	}
	return strings.Join(parts, ", ")
}

func dirOrCurrent(dest string) string {
	if strings.TrimSpace(dest) == "" {
		return "."
	}
	return dest
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
