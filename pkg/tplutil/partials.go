package tplutil

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// PartialsDir holds the pieces several templates share: a licence header, a
// CI job, a Makefile target.
//
// One sits at the root of the store and is reachable from every template; one
// sits inside a folder template and travels with it, shadowing the store's
// when both define the same name.
const PartialsDir = ".templates"

// LoadPartials returns the shared templates a render can reach, keyed by the
// name they are called with.
//
// A file `header.tmpl` in the folder is the partial `header`; one in a
// subfolder keeps its path, so `go/header.tmpl` is `go/header`.
func LoadPartials(tpl Template) (map[string]string, error) {
	partials := map[string]string{}

	dir, err := Dir()
	if err != nil {
		return nil, err
	}
	if err := collectPartials(filepath.Join(dir, PartialsDir), partials); err != nil {
		return nil, err
	}
	if tpl.IsDir() && tpl.Path != "" {
		// The template's own win: a project that brings its version of a
		// shared piece means to use that one.
		if err := collectPartials(filepath.Join(tpl.Path, PartialsDir), partials); err != nil {
			return nil, err
		}
	}
	return partials, nil
}

// collectPartials reads one folder of shared templates into the set.
//
// A folder that is not there means the store or the template simply shares
// nothing, which is the normal case.
func collectPartials(root string, into map[string]string) error {
	info, err := os.Stat(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("cannot access %s: %w", root, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("%s is a file, it must be a folder of shared templates", root)
	}

	return filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		relative, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("cannot read shared template %s: %w", path, err)
		}
		name := strings.TrimSuffix(filepath.ToSlash(relative), TemplateExt)
		into[name] = string(data)
		return nil
	})
}
