package tplutil

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// AddOptions drives `t add`, which copies an existing file or folder into the
// template store so it can be rendered later.
type AddOptions struct {
	// Source is the file or folder to import.
	Source string
	// Name is the name the template gets, defaulting to the source name.
	Name string
	// Force allows replacing a template that already exists.
	Force bool
	// Replace turns the literals a project repeats into template expressions,
	// in the file contents and in the file names alike.
	Replace []Replacement
}

// Added is what an import did.
type Added struct {
	// Path is where the template landed.
	Path string
	// Files is how many files were copied.
	Files int
	// Substitutions is how many literals were turned into template
	// expressions.
	Substitutions int
	// Manifest is the `.vars.yaml` the replacements implied, when there was
	// one to write.
	Manifest string
}

// Add copies the source into the template store and reports what it did.
//
// The content is copied as it is: Go template actions already in the file are
// kept, which is what makes `t add` the natural way to start a template. With
// --replace it is copied *and* parameterised, which is the other half of the
// job: a project says its own name in nineteen places, and every one of them
// has to become `{{ .Name }}` before it is a template.
func Add(o AddOptions) (Added, error) {
	source, err := normalizeDir(o.Source)
	if err != nil {
		return Added{}, err
	}
	info, err := os.Stat(source)
	if err != nil {
		return Added{}, fmt.Errorf("cannot add %s: %w", o.Source, err)
	}

	name := o.Name
	if name == "" {
		name = filepath.Base(source)
		if !info.IsDir() {
			// `main.go` becomes the template `main.go.tmpl`, so that rendering
			// it gives the file name back.
			name = strings.TrimSuffix(name, TemplateExt)
		}
	}
	if name != filepath.Base(name) || name == "." || name == ".." {
		return Added{}, fmt.Errorf("invalid template name %q", name)
	}

	dir, err := Dir()
	if err != nil {
		return Added{}, err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return Added{}, fmt.Errorf("cannot create template folder %s: %w", dir, err)
	}

	target := filepath.Join(dir, name)
	if !info.IsDir() {
		target += TemplateExt
	}
	if _, err := os.Stat(target); err == nil {
		if !o.Force {
			return Added{}, fmt.Errorf("template %q already exists at %s, use --force to replace it", name, target)
		}
		if err := os.RemoveAll(target); err != nil {
			return Added{}, fmt.Errorf("cannot replace %s: %w", target, err)
		}
	} else if !os.IsNotExist(err) {
		return Added{}, fmt.Errorf("cannot access %s: %w", target, err)
	}

	added := Added{Path: target}
	if info.IsDir() {
		// Importing the store into itself would recurse forever.
		if source == dir || strings.HasPrefix(source+string(os.PathSeparator), dir+string(os.PathSeparator)) {
			return Added{}, fmt.Errorf("cannot add %s: it is inside the template folder %s", source, dir)
		}
		files, substitutions, err := copyTree(source, target, o.Replace)
		if err != nil {
			return Added{}, err
		}
		added.Files, added.Substitutions = files, substitutions
	} else {
		substitutions, err := copyOne(source, target, info.Mode().Perm(), o.Replace)
		if err != nil {
			return Added{}, err
		}
		added.Files, added.Substitutions = 1, substitutions
	}

	manifest, err := manifestFor(o.Replace)
	if err != nil {
		return Added{}, err
	}
	if manifest != nil {
		path := filepath.Join(target, VarsFile)
		if !info.IsDir() {
			path = filepath.Join(dir, name+VarsFile)
		}
		if err := writeManifest(path, manifest, o.Force); err != nil {
			return Added{}, err
		}
		added.Manifest = path
	}
	return added, nil
}

// writeManifest writes the questions the replacements imply, leaving a
// manifest that is already there alone unless --force was given: it is more
// likely to be the real one than anything derived from a few literals.
func writeManifest(path string, content []byte, force bool) error {
	if !force {
		if _, err := os.Stat(path); err == nil {
			return nil
		} else if !os.IsNotExist(err) {
			return fmt.Errorf("cannot access %s: %w", path, err)
		}
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("cannot create folder %s: %w", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, content, 0o644); err != nil {
		return fmt.Errorf("cannot write %s: %w", path, err)
	}
	return nil
}

// ShowTemplate prints a template: the file itself, or every file of a folder
// template with a header naming it.
func ShowTemplate(out io.Writer, tpl Template) error {
	if !tpl.IsDir() {
		data, err := os.ReadFile(tpl.Path)
		if err != nil {
			return fmt.Errorf("cannot read template %s: %w", tpl.Path, err)
		}
		_, err = out.Write(data)
		return err
	}

	return filepath.WalkDir(tpl.Path, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		relative, err := filepath.Rel(tpl.Path, path)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("cannot read template file %s: %w", path, err)
		}
		if _, err := fmt.Fprintf(out, "# %s\n", relative); err != nil {
			return err
		}
		if _, err := out.Write(data); err != nil {
			return err
		}
		if len(data) > 0 && !strings.HasSuffix(string(data), "\n") {
			if _, err := fmt.Fprintln(out); err != nil {
				return err
			}
		}
		return nil
	})
}

// CopyTree copies a folder, leaving out the repository metadata: what is
// being copied is the files, not where they came from.
func CopyTree(source, target string) error {
	_, _, err := copyTree(source, target, nil)
	return err
}

// copyTree copies a folder, rewriting the literals as it goes, and reports how
// many files it wrote and how many substitutions it made.
func copyTree(source, target string, replacements []Replacement) (files, substitutions int, err error) {
	err = filepath.WalkDir(source, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		// The names repeat the literals too: a folder called after the project
		// has to become one called after `.Name`.
		renamed, renames := applyToPath(replacements, relative)
		destination := filepath.Join(target, renamed)

		if entry.IsDir() {
			// A .git folder is repository state, never part of a template.
			if relative != "." && entry.Name() == ".git" {
				return fs.SkipDir
			}
			substitutions += renames
			return os.MkdirAll(destination, 0o755)
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			// Symlinks and devices have no meaning once copied into the store.
			return nil
		}

		inside, err := copyOne(path, destination, info.Mode().Perm(), replacements)
		if err != nil {
			return err
		}
		files++
		substitutions += renames + inside
		return nil
	})
	return files, substitutions, err
}

// applyToPath rewrites each segment of a relative path on its own, so a
// replacement can never introduce a separator.
func applyToPath(replacements []Replacement, relative string) (string, int) {
	if len(replacements) == 0 || relative == "." {
		return relative, 0
	}

	segments := strings.Split(relative, string(os.PathSeparator))
	count := 0
	for i, segment := range segments {
		rewritten, n := apply(replacements, segment)
		segments[i], count = rewritten, count+n
	}
	return filepath.Join(segments...), count
}

// copyOne copies a file, rewriting its contents unless they are not text.
func copyOne(source, target string, mode os.FileMode, replacements []Replacement) (int, error) {
	data, err := os.ReadFile(source)
	if err != nil {
		return 0, fmt.Errorf("cannot read %s: %w", source, err)
	}

	count := 0
	if len(replacements) > 0 && isText(data) {
		var rewritten string
		rewritten, count = apply(replacements, string(data))
		data = []byte(rewritten)
	}

	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return 0, fmt.Errorf("cannot create folder %s: %w", filepath.Dir(target), err)
	}
	if err := os.WriteFile(target, data, mode); err != nil {
		return 0, fmt.Errorf("cannot write %s: %w", target, err)
	}
	return count, nil
}
