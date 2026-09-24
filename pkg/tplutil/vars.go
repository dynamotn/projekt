package tplutil

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"text/template/parse"

	"github.com/Masterminds/sprig"
	yaml "go.yaml.in/yaml/v3"
)

// VarsFile declares what a template asks for. It sits next to a file template
// as `<name>.vars.yaml`, and inside a folder template as `.vars.yaml`, where it
// travels with the folder and is never rendered.
const VarsFile = ".vars.yaml"

// VarType is how an answer is read back from the terminal.
type VarType string

const (
	// VarString keeps the answer as typed.
	VarString VarType = "string"
	// VarInt reads a whole number, and asks again when it is not one.
	VarInt VarType = "int"
	// VarBool reads yes/no, y/n, true/false, 1/0.
	VarBool VarType = "bool"
	// VarChoice accepts one of Choices.
	VarChoice VarType = "choice"
	// VarList reads a comma-separated list.
	VarList VarType = "list"
)

// Var is one value a template asks for.
type Var struct {
	// Name is the key under .Values, dotted to nest: "author.name".
	Name string `yaml:"name"`
	// Prompt is the question. It defaults to the name.
	Prompt string `yaml:"prompt"`
	// Default is a Go template rendered with the same context as the template
	// itself, minus .Values, so `{{ .User }}` works as a default.
	Default string `yaml:"default"`
	// Type is how the answer is read. The empty value means a string.
	Type VarType `yaml:"type"`
	// Choices are the accepted answers of a choice.
	Choices []string `yaml:"choices"`
	// Required asks again rather than accepting an empty answer.
	Required bool `yaml:"required"`
}

// question returns the text shown to the person answering.
func (v Var) question() string {
	if v.Prompt != "" {
		return v.Prompt
	}
	return v.Name
}

// Manifest is the parsed .vars.yaml of a template.
type Manifest struct {
	Vars []Var `yaml:"vars"`
	// Delims replaces `{{` and `}}` for the whole template.
	//
	// A template that writes Go templates, Helm charts or GitHub Actions
	// expressions otherwise spends its life escaping the very syntax it is
	// written in; saying `delims: ["<%", "%>"]` once is the way out.
	Delims []string `yaml:"delims"`
}

// DefaultDelims are the delimiters a template uses unless it says otherwise.
var DefaultDelims = [2]string{"{{", "}}"}

// delims returns the delimiters to parse this template with.
func (m Manifest) delims() [2]string {
	if len(m.Delims) != 2 {
		return DefaultDelims
	}
	return [2]string{m.Delims[0], m.Delims[1]}
}

// Delimiters returns the delimiters a template is written with, so that a
// question's default is rendered the same way the template would render it.
func Delimiters(tpl Template) ([2]string, error) {
	if tpl.Path == "" {
		return DefaultDelims, nil
	}
	manifest, err := LoadManifest(tpl)
	if err != nil {
		return DefaultDelims, err
	}
	return manifest.delims(), nil
}

// validateDelims refuses a pair that cannot work, rather than letting the
// template fail one file at a time.
func validateDelims(path string, delims []string) error {
	switch {
	case len(delims) == 0:
		return nil
	case len(delims) != 2:
		return fmt.Errorf("%s: delims takes exactly two values, the left and the right one", path)
	case strings.TrimSpace(delims[0]) == "" || strings.TrimSpace(delims[1]) == "":
		return fmt.Errorf("%s: neither delimiter may be empty", path)
	case delims[0] == delims[1]:
		return fmt.Errorf("%s: the two delimiters must differ, both are %q", path, delims[0])
	default:
		return nil
	}
}

// varsPath returns where a template's manifest lives.
func varsPath(tpl Template) string {
	if tpl.IsDir() {
		return filepath.Join(tpl.Path, VarsFile)
	}
	return filepath.Join(filepath.Dir(tpl.Path), tpl.Name+VarsFile)
}

// LoadManifest reads the template's .vars.yaml. A template without one is the
// normal case, and returns an empty manifest.
func LoadManifest(tpl Template) (Manifest, error) {
	path := varsPath(tpl)

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Manifest{}, nil
		}
		return Manifest{}, fmt.Errorf("cannot read %s: %w", path, err)
	}

	var manifest Manifest
	if err := yaml.Unmarshal(data, &manifest); err != nil {
		return Manifest{}, fmt.Errorf("cannot parse %s: %w", path, err)
	}
	if err := validateDelims(path, manifest.Delims); err != nil {
		return Manifest{}, err
	}
	for i, v := range manifest.Vars {
		if strings.TrimSpace(v.Name) == "" {
			return Manifest{}, fmt.Errorf("%s: variable at index %d has no name", path, i)
		}
		if v.Type == VarChoice && len(v.Choices) == 0 {
			return Manifest{}, fmt.Errorf("%s: variable %q is a choice with no choices", path, v.Name)
		}
	}
	return manifest, nil
}

// Vars returns what a template asks for: its manifest when it has one, and
// otherwise the `.Values` keys read out of the template itself, in the order
// they first appear.
func Vars(tpl Template) ([]Var, error) {
	manifest, err := LoadManifest(tpl)
	if err != nil {
		return nil, err
	}
	if len(manifest.Vars) > 0 {
		return manifest.Vars, nil
	}
	return InferVars(tpl)
}

// InferVars reads a template and reports the scalar `.Values` keys it uses.
//
// A key a template loops over holds a list or a map, which is not something to
// type at a prompt, so those are left out along with everything nested under
// them: they belong in a --values file.
func InferVars(tpl Template) ([]Var, error) {
	sources, err := templateSources(tpl)
	if err != nil {
		return nil, err
	}

	manifest, err := LoadManifest(tpl)
	if err != nil {
		return nil, err
	}
	delims := manifest.delims()

	scan := &varScan{seen: map[string]bool{}, containers: map[string]bool{}}
	for name, text := range sources {
		// The scan only has to parse, but a template calling `promptString`
		// does not parse at all unless the function is known, and one written
		// with its own delimiters does not parse without them either.
		t, err := template.New(name).Funcs(sprig.TxtFuncMap()).Funcs(parserFuncs()).
			Delims(delims[0], delims[1]).Option("missingkey=zero").Parse(text)
		if err != nil {
			return nil, fmt.Errorf("cannot parse template %s: %w", name, err)
		}
		if t.Tree != nil {
			scan.walk(t.Tree.Root, map[string]bool{})
		}
	}

	vars := make([]Var, 0, len(scan.order))
	for _, path := range scan.order {
		if scan.isContained(path) {
			continue
		}
		vars = append(vars, Var{Name: path})
	}
	return vars, nil
}

// templateSources returns every piece of a template that goes through the
// engine, keyed by a name usable in an error: the file contents, and for a
// folder template its path segments too.
func templateSources(tpl Template) (map[string]string, error) {
	sources := map[string]string{}

	if !tpl.IsDir() {
		data, err := os.ReadFile(tpl.Path)
		if err != nil {
			return nil, fmt.Errorf("cannot read template %s: %w", tpl.Path, err)
		}
		sources[tpl.Name] = string(data)
		return sources, nil
	}

	err := filepath.WalkDir(tpl.Path, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(tpl.Path, path)
		if err != nil {
			return err
		}
		if relative == "." {
			return nil
		}
		if skip, isDir := isReserved(entry, relative); skip {
			// The ignore file is the one piece of metadata that asks
			// something: `{{ if not .Values.ci }}` is a question about the
			// project, so it is read like any other file.
			if entry.Name() != IgnoreFile {
				if isDir {
					return fs.SkipDir
				}
				return nil
			}
		} else {
			// The path itself is a template too: `cmd/{{ .Name }}` names a
			// folder. Its attribute prefixes are literal, so they are read off
			// before it is scanned.
			bare, _ := ParseAttributes(filepath.Base(relative))
			sources["path:"+relative] = filepath.Join(filepath.Dir(relative), bare)
		}
		if entry.IsDir() {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("cannot read template file %s: %w", path, err)
		}
		sources[tpl.Name+"/"+relative] = string(data)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return sources, nil
}

// varScan collects the `.Values` paths a template reads.
type varScan struct {
	order []string
	seen  map[string]bool
	// containers are the paths a range or a with loops over. They hold
	// structure, not an answer.
	containers map[string]bool
}

// isContained reports whether a path is a container, or sits under one.
func (s *varScan) isContained(path string) bool {
	for container := range s.containers {
		if path == container || strings.HasPrefix(path, container+".") {
			return true
		}
	}
	return false
}

func (s *varScan) add(path string) {
	if path == "" || s.seen[path] {
		return
	}
	s.seen[path] = true
	s.order = append(s.order, path)
}

// walk visits a node, carrying the variables that alias .Values — the
// `{{ $v := .Values }}` a longer template opens with.
func (s *varScan) walk(node parse.Node, aliases map[string]bool) {
	switch n := node.(type) {
	case nil:
		return
	case *parse.ListNode:
		if n == nil {
			return
		}
		for _, child := range n.Nodes {
			s.walk(child, aliases)
		}
	case *parse.ActionNode:
		s.walkPipe(n.Pipe, aliases, false)
	case *parse.IfNode:
		s.walkBranch(&n.BranchNode, aliases, false)
	case *parse.WithNode:
		// `{{ with .Values.note }}` is the idiom for an optional scalar, so a
		// with is a plain read — unless its body loops over the scoped dot,
		// `{{ with .Values.items }}{{ range . }}`, which makes it a list.
		s.walkBranch(&n.BranchNode, aliases, loopsOverDot(n.List))
	case *parse.RangeNode:
		s.walkBranch(&n.BranchNode, aliases, true)
	case *parse.TemplateNode:
		s.walkPipe(n.Pipe, aliases, false)
	}
}

// loopsOverDot reports whether a body ranges over the value it was scoped to.
func loopsOverDot(list *parse.ListNode) bool {
	if list == nil {
		return false
	}
	for _, node := range list.Nodes {
		rangeNode, ok := node.(*parse.RangeNode)
		if !ok {
			continue
		}
		pipe := rangeNode.Pipe
		if pipe == nil || len(pipe.Cmds) != 1 || len(pipe.Cmds[0].Args) != 1 {
			continue
		}
		if _, isDot := pipe.Cmds[0].Args[0].(*parse.DotNode); isDot {
			return true
		}
	}
	return false
}

func (s *varScan) walkBranch(branch *parse.BranchNode, aliases map[string]bool, loops bool) {
	s.walkPipe(branch.Pipe, aliases, loops)
	// The dot inside the body is the element, not .Values, so only the
	// aliases still carry over.
	s.walk(branch.List, aliases)
	s.walk(branch.ElseList, aliases)
}

// walkPipe collects the paths of one pipeline. When the pipeline feeds a range
// or a with, its paths hold structure rather than a value to type.
func (s *varScan) walkPipe(pipe *parse.PipeNode, aliases map[string]bool, loops bool) {
	if pipe == nil {
		return
	}

	// `{{ $v := .Values }}` makes $v another way to spell .Values.
	if len(pipe.Decl) == 1 && len(pipe.Cmds) == 1 && len(pipe.Cmds[0].Args) == 1 {
		if field, ok := pipe.Cmds[0].Args[0].(*parse.FieldNode); ok &&
			len(field.Ident) == 1 && field.Ident[0] == "Values" {
			aliases[pipe.Decl[0].Ident[0]] = true
			return
		}
	}

	for _, cmd := range pipe.Cmds {
		for _, arg := range cmd.Args {
			path := valuesPath(arg, aliases)
			if path == "" {
				continue
			}
			if loops {
				s.containers[path] = true
				continue
			}
			s.add(path)
		}
		// A parenthesised sub-pipeline is an argument like any other.
		for _, arg := range cmd.Args {
			if sub, ok := arg.(*parse.PipeNode); ok {
				s.walkPipe(sub, aliases, false)
			}
		}
	}
}

// valuesPath returns the dotted key an argument reads under .Values, or "" when
// the argument reads something else.
func valuesPath(arg parse.Node, aliases map[string]bool) string {
	switch n := arg.(type) {
	case *parse.FieldNode:
		if len(n.Ident) >= 2 && n.Ident[0] == "Values" {
			return strings.Join(n.Ident[1:], ".")
		}
	case *parse.VariableNode:
		if len(n.Ident) >= 2 && aliases[n.Ident[0]] {
			return strings.Join(n.Ident[1:], ".")
		}
		// `$.Values.x` reaches the root context from inside a range.
		if len(n.Ident) >= 3 && n.Ident[0] == "$" && n.Ident[1] == "Values" {
			return strings.Join(n.Ident[2:], ".")
		}
	case *parse.ChainNode:
		if field, ok := n.Node.(*parse.FieldNode); ok &&
			len(field.Ident) == 1 && field.Ident[0] == "Values" && len(n.Field) > 0 {
			return strings.Join(n.Field, ".")
		}
	}
	return ""
}
