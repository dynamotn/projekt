package tplutil

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/Masterminds/sprig"
)

// engine is one rendering session.
//
// It carries the function set, the shared partials the store and the template
// bring, and the answers the prompt functions already collected, so the same
// question asked by ten files is asked once.
type engine struct {
	// root holds the partials, and is cloned for every file rendered.
	root *template.Template
	// source is the folder `include` resolves a relative path against: the
	// template itself, never anywhere above it.
	source string
	// in, out and ask are what the prompt functions read and write. Without
	// ask they take their default rather than stopping to ask.
	in  *bufio.Reader
	out io.Writer
	ask bool
	// answers keeps what a prompt function was already told.
	answers map[string]any
	// delims are the delimiters this template is written with. The shared
	// partials keep the default ones: they belong to the store, not to the
	// template calling them.
	delims [2]string
}

// newEngine builds the engine one render runs with.
func newEngine(o RenderOptions) (*engine, error) {
	e := &engine{
		source:  templateSource(o.Template),
		out:     o.Prompt,
		ask:     o.Interactive,
		answers: map[string]any{},
		delims:  DefaultDelims,
	}
	if o.Template.Path != "" {
		manifest, err := LoadManifest(o.Template)
		if err != nil {
			return nil, err
		}
		e.delims = manifest.delims()
	}
	if e.out == nil {
		e.out = os.Stderr
	}
	if o.In != nil {
		e.in = bufio.NewReader(o.In)
	} else {
		e.ask = false
	}

	e.root = template.New("").Funcs(sprig.TxtFuncMap()).Funcs(e.funcs()).Option("missingkey=zero")

	partials, err := LoadPartials(o.Template)
	if err != nil {
		return nil, err
	}
	for _, name := range sortedKeys(partials) {
		if _, err := e.root.New(name).Parse(partials[name]); err != nil {
			return nil, fmt.Errorf("cannot parse shared template %s: %w", name, err)
		}
	}
	return e, nil
}

// parserFuncs is the extra function set under a name, for the places that only
// need a template to parse rather than run.
func parserFuncs() template.FuncMap {
	return (&engine{answers: map[string]any{}}).funcs()
}

// templateSource returns the folder a template's own files live in.
func templateSource(tpl Template) string {
	if tpl.Path == "" {
		return ""
	}
	if tpl.IsDir() {
		return tpl.Path
	}
	return filepath.Dir(tpl.Path)
}

// execute parses and runs one template with the partials and the function set
// of the session.
func (e *engine) execute(name, text string, data map[string]any) ([]byte, error) {
	// A clone per file keeps one template's definitions out of the next one,
	// while every shared partial stays reachable.
	root, err := e.root.Clone()
	if err != nil {
		return nil, fmt.Errorf("cannot prepare template %s: %w", name, err)
	}
	// missingkey=zero keeps `{{ .Values.foo | default "bar" }}` working for
	// values the user did not set, instead of failing the whole render.
	t, err := root.New(name).Delims(e.delims[0], e.delims[1]).Parse(text)
	if err != nil {
		return nil, fmt.Errorf("cannot parse template %s: %w", name, err)
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("cannot render template %s: %w", name, err)
	}
	return buf.Bytes(), nil
}

// defaultEngine is the session a one-off piece of text runs with: the store's
// shared partials and the full function set, with nothing to ask questions on.
func defaultEngine() *engine {
	e, err := newEngine(RenderOptions{})
	if err != nil {
		// A broken partial must not stop a recipe's command line from being
		// rendered; it is reported where the partial is actually used.
		return &engine{
			root:    template.New("").Funcs(sprig.TxtFuncMap()).Option("missingkey=zero"),
			answers: map[string]any{},
			delims:  DefaultDelims,
		}
	}
	return e
}

// execute runs one piece of text outside a render: a manifest default, or a
// recipe's command line.
func execute(name, text string, data map[string]any) ([]byte, error) {
	return executeWith(DefaultDelims, name, text, data)
}

// executeWith runs one piece of text with the delimiters it was written in.
func executeWith(delims [2]string, name, text string, data map[string]any) ([]byte, error) {
	e := defaultEngine()
	e.delims = delims
	return e.execute(name, text, data)
}

// sortedKeys returns the keys of a map in a stable order, so that a parse
// error always points at the same partial.
func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	// A plain lexical order is enough: partials do not depend on each other at
	// parse time.
	for i := 1; i < len(keys); i++ {
		for j := i; j > 0 && keys[j] < keys[j-1]; j-- {
			keys[j], keys[j-1] = keys[j-1], keys[j]
		}
	}
	return keys
}

// insideSource reports whether a relative path stays within the template.
func insideSource(source, rel string) (string, error) {
	if source == "" {
		return "", fmt.Errorf("no template folder to read %q from", rel)
	}
	if filepath.IsAbs(rel) {
		return "", fmt.Errorf("%q must be relative to the template", rel)
	}
	target := filepath.Join(source, filepath.FromSlash(rel))
	clean := filepath.Clean(target)
	if clean != source && !strings.HasPrefix(clean, source+string(os.PathSeparator)) {
		return "", fmt.Errorf("%q reaches outside the template", rel)
	}
	return clean, nil
}
