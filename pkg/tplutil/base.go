package tplutil

import (
	"fmt"
	"os"
	"path/filepath"
)

// BaseDir is where a project keeps a copy of what each template last
// rendered, beside the record, so that a later apply has a third point to
// merge from: the file as it was written, the file as it is now, and the file
// as the template renders it today.
//
// Like the record, it is part of the project and worth committing: without it
// the next person's `t apply` can only tell "edited" from "not edited".
const BaseDir = "base"

// basePath returns where a project keeps the last render of a path.
func basePath(project, rel string) string {
	return filepath.Join(project, RecordDir, BaseDir, filepath.FromSlash(rel))
}

// loadBase returns what the template last rendered for a path, or nil when
// the project does not remember — one created before there was a base.
func loadBase(project, rel string) ([]byte, error) {
	data, err := os.ReadFile(basePath(project, rel))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("cannot read %s: %w", basePath(project, rel), err)
	}
	return data, nil
}

// saveBase remembers what the template rendered for a path.
func saveBase(project, rel string, content []byte) error {
	target := basePath(project, rel)
	if err := confinedTarget(project, target); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return fmt.Errorf("cannot create %s: %w", filepath.Dir(target), err)
	}
	if err := os.WriteFile(target, content, 0o644); err != nil {
		return fmt.Errorf("cannot write %s: %w", target, err)
	}
	return nil
}

// ensureBase writes the base when the project has none, or a different one,
// for a path whose file is already what the template says.
func ensureBase(project, rel string, content []byte) error {
	have, err := loadBase(project, rel)
	if err != nil {
		return err
	}
	if have != nil && string(have) == string(content) {
		return nil
	}
	return saveBase(project, rel, content)
}

// removeBase forgets a path the template no longer writes.
func removeBase(project, rel string) error {
	root := filepath.Join(project, RecordDir, BaseDir)
	return remove(root, basePath(project, rel))
}
