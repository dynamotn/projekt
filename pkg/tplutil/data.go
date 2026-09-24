package tplutil

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	yaml "go.yaml.in/yaml/v3"
)

// DataFile holds the values a template does not want to be asked for every
// time: a company name, a licence, a default registry.
//
// One sits at the root of the store and applies to every template; one sits
// inside a template and applies to that template alone. Both are below
// anything given on the command line, so a data file is a default, never an
// override.
const DataFile = ".data.yaml"

// dataExtensions are the spellings of a data file that are read, in the order
// they are tried. JSON is read by the YAML parser, being a subset of it.
var dataExtensions = []string{".yaml", ".yml", ".json"}

// LoadData returns the values a template starts with: the store's data file
// first, the template's own on top.
//
// A missing data file is the normal case and returns no values.
func LoadData(tpl Template) (Values, error) {
	values := Values{}

	dir, err := Dir()
	if err != nil {
		return nil, err
	}
	shared, err := readData(filepath.Join(dir, DataFile))
	if err != nil {
		return nil, err
	}
	values = MergeValues(values, shared)

	own, err := readData(dataPath(tpl))
	if err != nil {
		return nil, err
	}
	return MergeValues(values, own), nil
}

// WithData returns the values a render actually starts from: whatever the data
// files hold, with everything the user gave on top.
//
// It is what makes a data file answer a question instead of asking it: a value
// it sets is already there when --interactive walks the list.
func WithData(o RenderOptions) (Values, error) {
	data, err := LoadData(o.Template)
	if err != nil {
		return nil, err
	}
	return MergeValues(data, o.Values), nil
}

// dataPath returns where a template's own data file lives: inside a folder
// template, so it travels with it, and next to a file template.
func dataPath(tpl Template) string {
	switch {
	case tpl.Path == "":
		return ""
	case tpl.IsDir():
		return filepath.Join(tpl.Path, DataFile)
	default:
		return filepath.Join(filepath.Dir(tpl.Path), tpl.Name+DataFile)
	}
}

// readData reads one data file, trying every spelling of it.
func readData(base string) (Values, error) {
	if base == "" {
		return nil, nil
	}
	stem := strings.TrimSuffix(base, filepath.Ext(base))

	for _, ext := range dataExtensions {
		path := stem + ext
		data, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("cannot read %s: %w", path, err)
		}
		parsed := map[string]any{}
		if err := yaml.Unmarshal(data, &parsed); err != nil {
			return nil, fmt.Errorf("cannot parse %s: %w", path, err)
		}
		return Values(parsed), nil
	}
	return nil, nil
}

// isDataFile reports whether a name is a data file rather than something to
// render: `.data.yaml` inside a folder template, `<name>.data.yaml` next to a
// file template.
func isDataFile(name string) bool {
	stem := strings.TrimSuffix(DataFile, filepath.Ext(DataFile))
	for _, ext := range dataExtensions {
		if strings.HasSuffix(name, stem+ext) {
			return true
		}
	}
	return false
}
