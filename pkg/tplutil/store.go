package tplutil

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/OpenPeeDeeP/xdg"

	"gitlab.com/dynamo.foss/projekt/pkg/cli"
)

// TemplateExt is the suffix a template file may carry. It is stripped from the
// rendered file name, so `main.go.tmpl` renders to `main.go`.
const TemplateExt = ".tmpl"

// TemplateDir is the folder templates are read from. It is bound to the
// --template-dir flag; when empty the XDG data home is used.
var TemplateDir string

// Kind tells a single-file template apart from a folder of files.
type Kind string

const (
	// KindFile is a template made of one single file.
	KindFile Kind = "file"
	// KindDir is a template made of a whole folder tree.
	KindDir Kind = "dir"
)

// Template is one entry of the template store.
type Template struct {
	// Name is how the template is referred to on the command line.
	Name string
	// Path is the absolute path of the template file or folder.
	Path string
	// Kind is file for a single file, dir for a folder tree.
	Kind Kind
}

// IsDir reports whether the template renders a whole folder tree.
func (t Template) IsDir() bool {
	return t.Kind == KindDir
}

// DefaultTemplateDir returns the folder templates live in when no override is
// given: $XDG_DATA_HOME/projekt/templates.
func DefaultTemplateDir() string {
	return filepath.Join(xdg.DataHome(), "projekt", "templates")
}

// Dir returns the template folder to use, honouring --template-dir first and
// the PROJEKT_TEMPLATE_DIR environment variable second.
func Dir() (string, error) {
	dir := TemplateDir
	if dir == "" {
		dir = os.Getenv("PROJEKT_TEMPLATE_DIR")
	}
	if dir == "" {
		return DefaultTemplateDir(), nil
	}
	return normalizeDir(dir)
}

// normalizeDir expands a leading "~" and makes the path absolute, so the same
// folder is always spelled the same way in messages.
func normalizeDir(dir string) (string, error) {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return "", fmt.Errorf("template folder is empty")
	}
	if dir == "~" || strings.HasPrefix(dir, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("cannot resolve home directory: %w", err)
		}
		dir = filepath.Join(home, strings.TrimPrefix(dir, "~"))
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", fmt.Errorf("cannot resolve template folder %s: %w", dir, err)
	}
	return filepath.Clean(abs), nil
}

// List returns every template of the store, sorted by name.
//
// A missing store is not an error: it only means no template has been created
// yet, and every command reports that in its own way.
func List() ([]Template, error) {
	dir, err := Dir()
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			cli.Debug("Template folder does not exist: %s", dir)
			return nil, nil
		}
		return nil, fmt.Errorf("cannot read template folder %s: %w", dir, err)
	}

	templates := make([]Template, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		// Dotfiles are editor leftovers and VCS metadata, never templates.
		if strings.HasPrefix(name, ".") {
			continue
		}
		// A manifest and a data file describe the template they sit next to.
		if strings.HasSuffix(name, VarsFile) || isDataFile(name) {
			continue
		}
		if entry.IsDir() {
			templates = append(templates, Template{
				Name: name,
				Path: filepath.Join(dir, name),
				Kind: KindDir,
			})
			continue
		}
		templates = append(templates, Template{
			Name: strings.TrimSuffix(name, TemplateExt),
			Path: filepath.Join(dir, name),
			Kind: KindFile,
		})
	}

	sort.Slice(templates, func(i, j int) bool { return templates[i].Name < templates[j].Name })
	return templates, nil
}

// Get returns the template with the given name.
func Get(name string) (Template, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Template{}, fmt.Errorf("template name is empty")
	}
	// A name is a single store entry, never a path: "../../etc/passwd" must not
	// reach outside the store.
	if name != filepath.Base(name) || name == "." || name == ".." {
		return Template{}, fmt.Errorf("invalid template name %q", name)
	}

	templates, err := List()
	if err != nil {
		return Template{}, err
	}
	for _, tpl := range templates {
		if tpl.Name == name {
			return tpl, nil
		}
	}

	dir, dirErr := Dir()
	if dirErr != nil {
		return Template{}, dirErr
	}
	return Template{}, fmt.Errorf("template %q not found in %s", name, dir)
}
