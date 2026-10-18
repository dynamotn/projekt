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
	// ChangeMerged is a file the template writes differently *and* which was
	// edited by hand since, where the two sets of changes do not touch: the
	// template's are merged into the file, and the hand edits stay.
	ChangeMerged ChangeStatus = "merged"
	// ChangeConflict is a file the template writes differently *and* which was
	// edited by hand since, in a way that cannot be merged. Replacing it would
	// throw that away, so it takes --force.
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
	// Template is the store entry this change comes from, which matters once
	// a project is rendered from more than one.
	Template string
	// Attrs are what the template's name asked the file to be.
	Attrs Attributes
	// recorded is the hash the project remembers writing, which is what a
	// change the apply leaves alone must go on remembering.
	recorded string
	// render is what the template renders today, which is what the project
	// keeps as the base for the next merge. It is After for every change
	// but a merged one, where After is the merge.
	render []byte
}

// Writes reports whether this change touches the disk at all.
func (c Change) Writes() bool {
	return c.Status != ChangeUnchanged
}

// ApplyOptions drives `t diff` and `t apply`.
type ApplyOptions struct {
	// Templates are the folder templates to apply. Empty means every template
	// the project records, which is what makes `t apply` answerable without
	// being told what the project was made from.
	Templates []Template
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
	// Hooks runs the template's `after` commands again.
	//
	// Off by default, because those commands ran when the project was created
	// and repeating one unasked is how an apply loses somebody's trust.
	Hooks bool
	// Log receives what a hook is doing. Nil means nothing is said.
	Log io.Writer
	// HookOut and HookErr are the streams a hook runs with.
	HookOut, HookErr io.Writer
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
func PlanApply(o ApplyOptions) ([]Change, error) {
	root, err := projectRoot(o)
	if err != nil {
		return nil, err
	}
	templates, err := Targets(o)
	if err != nil {
		return nil, err
	}

	var changes []Change
	for _, tpl := range templates {
		planned, _, err := planOne(o, root, tpl)
		if err != nil {
			return nil, err
		}
		changes = append(changes, planned...)
	}
	return changes, nil
}

// Targets returns the templates an apply covers: the ones it was given, or
// everything the project remembers being rendered from.
func Targets(o ApplyOptions) ([]Template, error) {
	if len(o.Templates) > 0 {
		return o.Templates, nil
	}

	root, err := projectRoot(o)
	if err != nil {
		return nil, err
	}
	record, err := LoadRecord(root)
	if err != nil {
		return nil, err
	}
	if len(record.Renders) == 0 {
		return nil, fmt.Errorf("%s does not record being rendered from anything; name a template, or create it with `t new`", root)
	}

	templates := make([]Template, 0, len(record.Renders))
	for _, render := range record.Renders {
		tpl, err := Get(render.Template)
		if err != nil {
			return nil, fmt.Errorf("%s was rendered from %q: %w", root, render.Template, err)
		}
		templates = append(templates, tpl)
	}
	return templates, nil
}

// projectRoot resolves the folder an apply acts on.
func projectRoot(o ApplyOptions) (string, error) {
	root, err := filepath.Abs(dirOrCurrent(o.Dest))
	if err != nil {
		return "", fmt.Errorf("cannot resolve %s: %w", o.Dest, err)
	}
	return root, nil
}

// planOne works out what one template would do to the project.
func planOne(o ApplyOptions, root string, template Template) ([]Change, RenderRecord, error) {
	if !template.IsDir() {
		return nil, RenderRecord{}, fmt.Errorf("%s is a file template; only a folder template can be applied", template.Name)
	}

	record, err := LoadRecord(root)
	if err != nil {
		return nil, RenderRecord{}, err
	}
	previous, remembered := record.Find(template.Name)

	// The recorded values are replayed, and anything given now wins over them,
	// so `t apply go-cli . --set ci=true` changes one answer and keeps the
	// rest.
	render := RenderOptions{
		Template:    template,
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
		Template:   template.Name,
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
		var base []byte
		if !file.Attrs.Symlink {
			if base, err = loadBase(root, file.Rel); err != nil {
				return nil, RenderRecord{}, err
			}
		}
		status, after := decide(before, file.Content, base, previous.Files[file.Rel], remembered)
		changes = append(changes, Change{
			Path:     file.Rel,
			Status:   status,
			Before:   before,
			After:    after,
			Template: template.Name,
			Attrs:    file.Attrs,
			recorded: previous.Files[file.Rel],
			render:   file.Content,
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
		changes = append(changes, Change{Path: path, Status: status, Before: before, Template: template.Name, recorded: hash})
	}

	sort.Slice(changes, func(i, j int) bool { return changes[i].Path < changes[j].Path })
	return changes, next, nil
}

// decide works out what one file's change is, and what the file should
// become.
//
// Two questions settle it. Has the file been touched since the last apply —
// the record's hash says. Has the template changed since — the base, the copy
// of the last render, says. When both moved, the base is what lets the two be
// merged; a project from before there was a base falls back to telling
// "touched" from "not touched" alone.
func decide(before, after, base []byte, recorded string, remembered bool) (ChangeStatus, []byte) {
	if before == nil {
		return ChangeAdded, after
	}
	if string(before) == string(after) {
		return ChangeUnchanged, after
	}

	touched := !remembered || hashOf(before) != recorded
	templateChanged := base == nil || string(base) != string(after)

	switch {
	case !templateChanged:
		if !touched {
			// The file is the last merge, and the template has nothing new.
			return ChangeUnchanged, after
		}
		// Edited by hand, and the template has nothing to say about it.
		return ChangeConflict, after
	case !touched && (base == nil || string(before) == string(base)):
		// Exactly what was written last time, so it is ours to replace.
		return ChangeUpdated, after
	case base == nil:
		return ChangeConflict, after
	}

	// The file carries edits — by hand, or from an earlier merge — and the
	// template moved too. Bring the template's change in around them.
	merged, ok := merge3(base, before, after)
	if !ok {
		return ChangeConflict, after
	}
	if string(merged) == string(before) {
		// The template's change was already made by hand.
		return ChangeUnchanged, after
	}
	return ChangeMerged, merged
}

// Apply renders a template over a project it was already rendered into.
//
// Nothing that was edited by hand is touched without --force, and nothing is
// deleted without --prune: an apply that quietly threw work away would only be
// run once.
func Apply(o ApplyOptions) ([]Change, error) {
	root, err := projectRoot(o)
	if err != nil {
		return nil, err
	}
	templates, err := Targets(o)
	if err != nil {
		return nil, err
	}

	var all []Change
	for _, template := range templates {
		changes, next, err := planOne(o, root, template)
		if err != nil {
			return nil, err
		}
		all = append(all, changes...)
		if o.DryRun && !o.Hooks {
			continue
		}
		if !o.DryRun {
			if err := applyOne(o, root, changes, next); err != nil {
				return nil, err
			}
		}
		if o.Hooks {
			if err := runTemplateHooks(o, root, template, next.Values); err != nil {
				return nil, err
			}
		}
	}
	return all, nil
}

// runTemplateHooks runs what the template's own manifest asks for, in the
// project that was just brought up to date.
func runTemplateHooks(o ApplyOptions, root string, template Template, values Values) error {
	base, err := BaseContext(RenderOptions{Template: template, Dest: root, Values: values})
	if err != nil {
		return err
	}
	return RunAfter(HookOptions{
		Template: template,
		Dir:      root,
		Context:  base,
		DryRun:   o.DryRun,
		Log:      o.Log,
		In:       o.In,
		Out:      o.HookOut,
		Err:      o.HookErr,
	})
}

// applyOne writes what one template's plan asked for, and records it.
func applyOne(o ApplyOptions, root string, changes []Change, next RenderRecord) error {
	for _, change := range changes {
		target := filepath.Join(root, filepath.FromSlash(change.Path))
		switch change.Status {
		case ChangeUnchanged:
			// A project made before there was a base gains one here, so its
			// next apply can merge too.
			if change.render != nil && !change.Attrs.Symlink {
				if err := ensureBase(root, change.Path, change.render); err != nil {
					return err
				}
			}
			continue
		case ChangeAdded, ChangeUpdated:
			if err := confinedTarget(root, target); err != nil {
				return err
			}
			if err := writeFile(target, change.After, true, change.Attrs); err != nil {
				return err
			}
			if err := saveBase(root, change.Path, change.render); err != nil {
				return err
			}
		case ChangeMerged:
			if err := confinedTarget(root, target); err != nil {
				return err
			}
			if err := writeFile(target, change.After, true, change.Attrs); err != nil {
				return err
			}
			// What is on disk is the merge; what the next merge starts from
			// is the template's own render.
			next.Files[change.Path] = hashOf(change.After)
			if err := saveBase(root, change.Path, change.render); err != nil {
				return err
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
					return err
				}
				if err := removeBase(root, change.Path); err != nil {
					return err
				}
				continue
			}
			if err := confinedTarget(root, target); err != nil {
				return err
			}
			if err := writeFile(target, change.After, true, change.Attrs); err != nil {
				return err
			}
			if err := saveBase(root, change.Path, change.render); err != nil {
				return err
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
				return err
			}
			if err := removeBase(root, change.Path); err != nil {
				return err
			}
		}
	}

	return recordRender(root, next)
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
	for _, status := range []ChangeStatus{ChangeAdded, ChangeUpdated, ChangeMerged, ChangeConflict, ChangeRemoved, ChangeUnchanged} {
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
