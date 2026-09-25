package tplutil

import (
	"fmt"
	"sort"
	"strings"
	"text/template"
	"text/template/parse"

	"github.com/Masterminds/sprig"

	"gitlab.com/dynamo.foss/projekt/pkg/lazypath"
)

// Report is what checking one template found.
type Report struct {
	// Template is the store entry the report is about.
	Template Template
	// Diagnostics are the problems found, in the order they were found.
	Diagnostics []lazypath.Diagnostic
}

// Errors counts the problems that make the template unusable.
func (r Report) Errors() int {
	count := 0
	for _, diag := range r.Diagnostics {
		if diag.Severity == lazypath.SeverityError {
			count++
		}
	}
	return count
}

// Warnings counts the problems that only look wrong.
func (r Report) Warnings() int {
	return len(r.Diagnostics) - r.Errors()
}

// checker collects what is wrong with one template.
type checker struct {
	tpl   Template
	diags []lazypath.Diagnostic
}

func (c *checker) errorf(format string, v ...any) {
	c.diags = append(c.diags, lazypath.Diagnostic{Severity: lazypath.SeverityError, Message: fmt.Sprintf(format, v...)})
}

func (c *checker) warnf(format string, v ...any) {
	c.diags = append(c.diags, lazypath.Diagnostic{Severity: lazypath.SeverityWarning, Message: fmt.Sprintf(format, v...)})
}

// Check reads a template the way rendering would, and reports everything that
// would go wrong, rather than the first thing.
//
// A template now carries four files that describe it and a folder of shared
// pieces; the chance of a typo went up with them, and a typo found before the
// render is one nobody has to undo afterwards.
func Check(tpl Template) Report {
	c := &checker{tpl: tpl}

	manifest, err := LoadManifest(tpl)
	if err != nil {
		// Nothing else can be trusted once the manifest is unreadable: the
		// delimiters and the defaults both come out of it.
		c.errorf("%v", err)
		return Report{Template: tpl, Diagnostics: c.diags}
	}
	if _, err := LoadData(tpl); err != nil {
		c.errorf("%v", err)
	}

	partials, err := LoadPartials(tpl)
	if err != nil {
		c.errorf("%v", err)
		partials = map[string]string{}
	}
	c.checkPartials(partials)

	delims := manifest.delims()
	read, askable := c.checkSources(partials, delims)
	c.checkManifest(manifest, delims)
	c.checkVars(manifest, read, askable)
	c.checkRender(manifest)

	return Report{Template: tpl, Diagnostics: c.diags}
}

// CheckAll checks every template of the store.
func CheckAll() ([]Report, error) {
	templates, err := List()
	if err != nil {
		return nil, err
	}
	reports := make([]Report, 0, len(templates))
	for _, tpl := range templates {
		reports = append(reports, Check(tpl))
	}
	return reports, nil
}

// checkPartials makes sure the shared pieces parse, since a template calling
// one fails at parse time otherwise.
func (c *checker) checkPartials(partials map[string]string) {
	for _, name := range sortedKeys(partials) {
		if _, err := parseOnly(name, partials[name], DefaultDelims); err != nil {
			c.errorf("shared template %s: %v", name, err)
		}
	}
}

// checkSources parses every file and every path segment, and reports the
// shared templates they call but nobody defines.
//
// It returns every `.Values` key the template reads, and the subset of them a
// manifest could sensibly ask for: a list or a map is not something to type at
// a prompt, and belongs in a --values file.
func (c *checker) checkSources(partials map[string]string, delims [2]string) (read, askable map[string]bool) {
	read, askable = map[string]bool{}, map[string]bool{}

	sources, err := templateSources(c.tpl)
	if err != nil {
		c.errorf("%v", err)
		return read, askable
	}

	names := make([]string, 0, len(sources))
	for name := range sources {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		tree, err := parseOnly(name, sources[name], delims)
		if err != nil {
			c.errorf("%v", err)
			continue
		}
		for _, called := range templateCalls(tree) {
			if _, ok := partials[called]; !ok {
				c.errorf("%s calls the shared template %q, which is not in any %s folder", name, called, PartialsDir)
			}
		}
	}

	use, err := ScanValues(c.tpl)
	if err != nil {
		return read, askable
	}
	read = use.Read
	for name := range use.Askable {
		// A value the template already handles the absence of is optional by
		// design; a manifest that does not ask for it is not an oversight.
		if !use.Optional[name] {
			askable[name] = true
		}
	}
	return read, askable
}

// checkManifest looks at what the manifest itself says.
func (c *checker) checkManifest(manifest Manifest, delims [2]string) {
	seen := map[string]bool{}
	for _, v := range manifest.Vars {
		if seen[v.Name] {
			c.warnf("%s asks for %q twice", VarsFile, v.Name)
		}
		seen[v.Name] = true

		if v.Default == "" {
			continue
		}
		if _, err := parseOnly("default:"+v.Name, v.Default, delims); err != nil {
			c.errorf("%v", err)
		}
	}

	for i, command := range manifest.After {
		if _, err := parseOnly(fmt.Sprintf("after[%d]", i), command, delims); err != nil {
			c.errorf("%v", err)
		}
	}
}

// checkVars compares what the manifest asks for with what the template reads.
//
// Both directions are worth saying: a question nobody uses is noise at the
// prompt, and a value nobody asks about is one somebody will forget to pass.
func (c *checker) checkVars(manifest Manifest, read, askable map[string]bool) {
	if len(manifest.Vars) == 0 {
		return
	}

	// A value a data file already holds is answered, not unasked: it is never
	// missing at render time and never shows up at the prompt.
	declared := map[string]bool{}
	if keys, err := DataKeys(c.tpl); err == nil {
		declared = keys
	}

	for _, v := range manifest.Vars {
		declared[v.Name] = true
		if !read[v.Name] && !readsUnder(read, v.Name) {
			c.warnf("%s asks for %q, which the template never reads", VarsFile, v.Name)
		}
	}
	// Only the answerable ones: a list the template loops over is meant to
	// come from a --values file, not from a question.
	missing := make([]string, 0, len(askable))
	for name := range askable {
		if declared[name] || declaredAbove(declared, name) {
			continue
		}
		missing = append(missing, name)
	}
	sort.Strings(missing)
	for _, name := range missing {
		c.warnf("the template reads .Values.%s, which %s never asks for, so --interactive will not offer it", name, VarsFile)
	}
}

// readsUnder reports whether anything nested under a declared name is read,
// so declaring `author` and using `.Values.author.name` is not a complaint.
func readsUnder(used map[string]bool, name string) bool {
	for read := range used {
		if strings.HasPrefix(read, name+".") {
			return true
		}
	}
	return false
}

// declaredAbove is the same question the other way round.
func declaredAbove(declared map[string]bool, name string) bool {
	for declaration := range declared {
		if strings.HasPrefix(name, declaration+".") {
			return true
		}
	}
	return false
}

// checkRender renders the whole template with its own defaults, which is the
// only way to catch a path segment that comes out empty or a name that would
// escape the destination.
func (c *checker) checkRender(manifest Manifest) {
	if !c.tpl.IsDir() {
		return
	}

	options := RenderOptions{
		Template: c.tpl,
		Dest:     "/nonexistent-check",
		Name:     "example",
		Values:   c.defaults(manifest),
		lenient:  true,
	}
	files, err := Collect(options)
	if err != nil {
		c.errorf("%v", err)
		return
	}

	written := 0
	for _, file := range files {
		if !file.IsDir {
			written++
		}
	}
	if written == 0 {
		c.warnf("the template writes nothing: %s leaves every file out", IgnoreFile)
	}
}

// defaults builds the values a template would have if every question were
// answered with its default, which is what a check should judge it on.
func (c *checker) defaults(manifest Manifest) Values {
	values := Values{}
	base := map[string]any{"Name": "example", "User": "example", "Values": map[string]any{}}
	prompter := Prompter{Delims: manifest.delims()}

	for _, v := range manifest.Vars {
		rendered, err := prompter.renderDefault(v, base)
		if err != nil || rendered == "" {
			continue
		}
		parsed, err := parseAnswer(v, rendered)
		if err != nil {
			c.warnf("the default of %q is not a valid %s: %v", v.Name, v.typeName(), err)
			continue
		}
		if err := assign(values, strings.Split(v.Name, "."), parsed); err != nil {
			c.warnf("%s: %v", v.Name, err)
		}
	}
	return values
}

// parseOnly parses a piece of a template without running it, which is what a
// check can do for a template whose values it does not have.
func parseOnly(name, text string, delims [2]string) (*template.Template, error) {
	t, err := template.New(name).Funcs(sprig.TxtFuncMap()).Funcs(parserFuncs()).
		Delims(delims[0], delims[1]).Option("missingkey=zero").Parse(text)
	if err != nil {
		return nil, fmt.Errorf("cannot parse %s: %w", name, err)
	}
	return t, nil
}

// templateCalls returns the shared templates a parsed template refers to,
// whether with `{{ template "x" }}` or `{{ includeTemplate "x" . }}`.
func templateCalls(t *template.Template) []string {
	if t == nil || t.Tree == nil {
		return nil
	}
	seen := map[string]bool{}
	var names []string
	add := func(name string) {
		if name == "" || seen[name] {
			return
		}
		seen[name] = true
		names = append(names, name)
	}

	var walk func(parse.Node)
	walk = func(node parse.Node) {
		switch n := node.(type) {
		case nil:
			return
		case *parse.ListNode:
			if n == nil {
				return
			}
			for _, child := range n.Nodes {
				walk(child)
			}
		case *parse.TemplateNode:
			add(n.Name)
		case *parse.IfNode:
			walk(n.List)
			walk(n.ElseList)
		case *parse.WithNode:
			walk(n.List)
			walk(n.ElseList)
		case *parse.RangeNode:
			walk(n.List)
			walk(n.ElseList)
		case *parse.ActionNode:
			for _, name := range includeTemplateCalls(n.Pipe) {
				add(name)
			}
		}
	}
	walk(t.Tree.Root)
	return names
}

// includeTemplateCalls finds `includeTemplate "name"` in one pipeline.
func includeTemplateCalls(pipe *parse.PipeNode) []string {
	if pipe == nil {
		return nil
	}
	var names []string
	for _, cmd := range pipe.Cmds {
		for i, arg := range cmd.Args {
			if sub, ok := arg.(*parse.PipeNode); ok {
				names = append(names, includeTemplateCalls(sub)...)
				continue
			}
			ident, ok := arg.(*parse.IdentifierNode)
			if !ok || ident.Ident != "includeTemplate" || i+1 >= len(cmd.Args) {
				continue
			}
			if literal, ok := cmd.Args[i+1].(*parse.StringNode); ok {
				names = append(names, literal.Text)
			}
		}
	}
	return names
}
