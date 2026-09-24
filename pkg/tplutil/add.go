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
}

// Add copies the source into the template store and returns where it landed.
//
// The content is copied as it is: Go template actions already in the file are
// kept, which is what makes `t add` the natural way to start a template.
func Add(o AddOptions) (string, error) {
	source, err := normalizeDir(o.Source)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(source)
	if err != nil {
		return "", fmt.Errorf("cannot add %s: %w", o.Source, err)
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
		return "", fmt.Errorf("invalid template name %q", name)
	}

	dir, err := Dir()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("cannot create template folder %s: %w", dir, err)
	}

	target := filepath.Join(dir, name)
	if !info.IsDir() {
		target += TemplateExt
	}
	if _, err := os.Stat(target); err == nil {
		if !o.Force {
			return "", fmt.Errorf("template %q already exists at %s, use --force to replace it", name, target)
		}
		if err := os.RemoveAll(target); err != nil {
			return "", fmt.Errorf("cannot replace %s: %w", target, err)
		}
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("cannot access %s: %w", target, err)
	}

	if info.IsDir() {
		// Importing the store into itself would recurse forever.
		if source == dir || strings.HasPrefix(source+string(os.PathSeparator), dir+string(os.PathSeparator)) {
			return "", fmt.Errorf("cannot add %s: it is inside the template folder %s", source, dir)
		}
		if err := CopyTree(source, target); err != nil {
			return "", err
		}
		return target, nil
	}
	if err := copyFile(source, target, info.Mode().Perm()); err != nil {
		return "", err
	}
	return target, nil
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
	return filepath.WalkDir(source, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		destination := filepath.Join(target, relative)
		if entry.IsDir() {
			// A .git folder is repository state, never part of a template.
			if relative != "." && entry.Name() == ".git" {
				return fs.SkipDir
			}
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
		return copyFile(path, destination, info.Mode().Perm())
	})
}

func copyFile(source, target string, mode os.FileMode) error {
	data, err := os.ReadFile(source)
	if err != nil {
		return fmt.Errorf("cannot read %s: %w", source, err)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return fmt.Errorf("cannot create folder %s: %w", filepath.Dir(target), err)
	}
	if err := os.WriteFile(target, data, mode); err != nil {
		return fmt.Errorf("cannot write %s: %w", target, err)
	}
	return nil
}
