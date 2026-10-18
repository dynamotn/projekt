package tplutil

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"text/template"

	"github.com/Masterminds/sprig/v3"
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
	// lenient lets an unanswerable question render empty, for a check that
	// only wants to know whether the template holds together.
	lenient bool
	// origin is where the text being rendered comes from, which decides
	// whether `output` may run a command.
	origin Origin
	// delims are the delimiters this template is written with. The shared
	// partials keep the default ones: they belong to the store, not to the
	// template calling them.
	delims [2]string
	// noExec keeps `output` and `env` from doing anything: a check or a dry
	// run looks at a template, it does not act on its behalf.
	noExec bool
	// static is the part of the data every file is rendered with that does
	// not depend on the file, built once per session.
	static map[string]any
	// env is the environment the template may see, worked out once.
	env     map[string]string
	envOnce bool
}

// newEngine builds the engine one render runs with.
func newEngine(o RenderOptions) (*engine, error) {
	e := &engine{
		source:  templateSource(o.Template),
		out:     o.Prompt,
		ask:     o.Interactive,
		answers: map[string]any{},
		lenient: o.lenient,
		noExec:  o.lenient || o.DryRun,
		delims:  DefaultDelims,
		origin:  o.Origin,
	}
	if e.origin.key() == "" {
		e.origin = TemplateOrigin(o.Template)
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

	e.root = template.New("").Funcs(sprigFuncs()).Funcs(e.funcs()).Option("missingkey=zero")

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
	return (&engine{answers: map[string]any{}, noExec: true}).funcs()
}

// sprigFuncs is the sprig function set as a template gets it.
//
// Three of sprig's functions are left out. `env` and `expandenv` come back
// under the same names from the engine, where reading the environment is a
// trust decision like running a command: a template somebody else wrote
// must not copy a token out of the shell into a file. `getHostByName` does a
// DNS lookup, which is a way to tell a server a template was rendered, and
// no scaffolding needs it.
func sprigFuncs() template.FuncMap {
	funcs := sprig.TxtFuncMap()
	delete(funcs, "env")
	delete(funcs, "expandenv")
	delete(funcs, "getHostByName")
	return funcs
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
func defaultEngine(origin Origin, noExec bool) *engine {
	e, err := newEngine(RenderOptions{Origin: origin, DryRun: noExec})
	if err != nil {
		// A broken partial must not stop a recipe's command line from being
		// rendered; it is reported where the partial is actually used.
		e = &engine{
			answers: map[string]any{},
			delims:  DefaultDelims,
			origin:  origin,
			noExec:  noExec,
		}
		e.root = template.New("").Funcs(sprigFuncs()).Funcs(e.funcs()).Option("missingkey=zero")
	}
	return e
}

// execute runs one piece of text outside a render: a manifest default, or a
// recipe's command line.
func execute(origin Origin, name, text string, data map[string]any) ([]byte, error) {
	return executeWith(origin, DefaultDelims, name, text, data)
}

// executeWith runs one piece of text with the delimiters it was written in.
func executeWith(origin Origin, delims [2]string, name, text string, data map[string]any) ([]byte, error) {
	return executeMode(origin, delims, name, text, data, false)
}

// executeMode is executeWith with a say on whether the text may run
// commands: a line only being looked at must not.
func executeMode(origin Origin, delims [2]string, name, text string, data map[string]any, noExec bool) ([]byte, error) {
	e := defaultEngine(origin, noExec)
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
//
// The name is checked here; whether a link on disk carries it elsewhere is
// checked when it is read.
func insideSource(source, rel string) (string, error) {
	if source == "" {
		return "", fmt.Errorf("no template folder to read %q from", rel)
	}
	if filepath.IsAbs(rel) {
		return "", fmt.Errorf("%q must be relative to the template", rel)
	}
	target := filepath.Join(source, filepath.FromSlash(rel))
	clean := filepath.Clean(target)
	if !within(filepath.Clean(source), clean) {
		return "", fmt.Errorf("%q reaches outside the template", rel)
	}
	return clean, nil
}
