package bplutil

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	yaml "go.yaml.in/yaml/v3"

	"gitlab.com/dynamo-tools/projekt/pkg/folderutil"
	"gitlab.com/dynamo-tools/projekt/pkg/lazypath"
	"gitlab.com/dynamo-tools/projekt/pkg/tplutil"
)

// Report is what checking one recipe found.
type Report struct {
	// Name is the recipe the report is about.
	Name string
	// Path is the file it was read from.
	Path string
	// Diagnostics are the problems found, in the order they were found.
	Diagnostics []lazypath.Diagnostic
}

// Errors counts the problems that make the recipe unusable.
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

// checker collects what is wrong with one recipe.
type checker struct {
	diags []lazypath.Diagnostic
}

func (c *checker) errorf(format string, v ...any) {
	c.diags = append(c.diags, lazypath.Diagnostic{Severity: lazypath.SeverityError, Message: fmt.Sprintf(format, v...)})
}

func (c *checker) warnf(format string, v ...any) {
	c.diags = append(c.diags, lazypath.Diagnostic{Severity: lazypath.SeverityWarning, Message: fmt.Sprintf(format, v...)})
}

// Check reads a recipe and reports everything wrong with it, rather than the
// first thing.
//
// Loading a recipe by name already refuses a broken one; a check has to read
// it anyway, because a report that stops at the first problem is a report you
// have to run four times.
func Check(name string) Report {
	c := &checker{}
	report := Report{Name: name}

	path, err := recipePath(name)
	if err != nil {
		c.errorf("%v", err)
		return Report{Name: name, Diagnostics: c.diags}
	}
	report.Path = path

	recipe, err := readRecipe(path, name)
	if err != nil {
		c.errorf("%v", err)
		return Report{Name: name, Path: path, Diagnostics: c.diags}
	}

	c.checkSource(recipe)
	c.checkVars(recipe)
	c.checkAfter(recipe)
	c.checkRegister(recipe)

	report.Diagnostics = c.diags
	return report
}

// CheckAll checks every recipe of the store, the ones too broken to load
// included.
func CheckAll() ([]Report, error) {
	names, err := recipeNames()
	if err != nil {
		return nil, err
	}
	reports := make([]Report, 0, len(names))
	for _, name := range names {
		reports = append(reports, Check(name))
	}
	return reports, nil
}

// recipeNames lists the store by file name, so a recipe that does not parse is
// still checked rather than quietly skipped.
func recipeNames() ([]string, error) {
	dir, err := Dir()
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("cannot read boilerplate folder %s: %w", dir, err)
	}

	var names []string
	for _, entry := range entries {
		if entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		if name := recipeName(entry.Name()); name != "" {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names, nil
}

// recipePath returns the file a recipe is read from.
func recipePath(name string) (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	for _, ext := range []string{".yaml", ".yml"} {
		path := filepath.Join(dir, name+ext)
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}
	return "", fmt.Errorf("boilerplate %q not found in %s", name, dir)
}

// readRecipe parses a recipe without validating it, which is what lets the
// check report every problem instead of the first.
func readRecipe(path, name string) (Recipe, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Recipe{}, fmt.Errorf("cannot read %s: %w", path, err)
	}
	var recipe Recipe
	if err := yaml.Unmarshal(data, &recipe); err != nil {
		return Recipe{}, fmt.Errorf("cannot parse %s: %w", path, err)
	}
	recipe.Name, recipe.Path = name, path
	return recipe, nil
}

// checkSource makes sure there is exactly one origin and that it is reachable.
func (c *checker) checkSource(recipe Recipe) {
	template, repo := recipe.Source.Template != "", recipe.Source.Repo != ""
	switch {
	case !template && !repo:
		c.errorf("source declares nothing to create from, expected source.template or source.repo")
		return
	case template && repo:
		c.errorf("source declares more than one origin, expected exactly one")
		return
	}

	if repo {
		if _, err := folderutil.ParseRepoRef(recipe.Source.Repo); err != nil {
			c.errorf("%v", err)
		}
		return
	}

	tpl, err := tplutil.Get(recipe.Source.Template)
	if err != nil {
		c.errorf("source.template: %v", err)
		return
	}
	// The recipe creates a project, and a project is a folder.
	if !tpl.IsDir() {
		c.warnf("source.template %q is a file template, so the project is a single file", tpl.Name)
	}

	// Everything `t check` says about the template is worth saying here too:
	// a recipe is only as good as what it renders.
	if report := tplutil.Check(tpl); report.Errors() > 0 {
		c.errorf("source.template %q does not check out; run `t check %s`", tpl.Name, tpl.Name)
	}
}

// checkVars looks at the questions the recipe asks, and at the ones it should.
func (c *checker) checkVars(recipe Recipe) {
	seen := map[string]bool{}
	for i, v := range recipe.Vars {
		switch {
		case strings.TrimSpace(v.Name) == "":
			c.errorf("the variable at index %d has no name", i)
			continue
		case v.Type == tplutil.VarChoice && len(v.Choices) == 0:
			c.errorf("variable %q is a choice with no choices", v.Name)
		}
		if seen[v.Name] {
			c.warnf("the recipe asks for %q twice", v.Name)
		}
		seen[v.Name] = true
	}

	if len(recipe.Vars) == 0 || recipe.Source.Template == "" {
		return
	}
	tpl, err := tplutil.Get(recipe.Source.Template)
	if err != nil {
		return
	}
	use, err := tplutil.ScanValues(tpl)
	if err != nil {
		return
	}

	// A recipe's vars replace the template's, so a question the template has
	// no use for only slows the person answering it down.
	unread := make([]string, 0, len(seen))
	for name := range seen {
		if use.Read[name] || readsUnder(use.Read, name) {
			continue
		}
		unread = append(unread, name)
	}
	sort.Strings(unread)
	for _, name := range unread {
		c.warnf("the recipe asks for %q, which the template %s never reads", name, tpl.Name)
	}

	// The other direction, against what is actually still unanswered: a value
	// a data file already holds never reaches a prompt to begin with.
	answered := map[string]bool{}
	if keys, err := tplutil.DataKeys(tpl); err == nil {
		answered = keys
	}
	for name := range seen {
		answered[name] = true
	}

	missing := make([]string, 0, len(use.Askable))
	for name := range use.Askable {
		if answered[name] || use.Optional[name] || declaredAbove(answered, name) {
			continue
		}
		missing = append(missing, name)
	}
	sort.Strings(missing)
	for _, name := range missing {
		c.warnf("the template %s reads .Values.%s, which the recipe never asks for", tpl.Name, name)
	}
}

// checkAfter makes sure every command line is a template that parses.
func (c *checker) checkAfter(recipe Recipe) {
	for i, command := range recipe.After {
		if strings.TrimSpace(command) == "" {
			c.warnf("after[%d] is empty", i)
			continue
		}
		if _, err := tplutil.RenderString(fmt.Sprintf("%s:after[%d]", recipe.Name, i), command, checkContext); err != nil {
			c.errorf("%v", err)
		}
	}
}

// checkRegister looks at where the project would go and what it would point at.
func (c *checker) checkRegister(recipe Recipe) {
	if workspace := strings.TrimSpace(recipe.Register.Workspace); workspace != "" {
		path, err := lazypath.NormalizePath(workspace)
		if err != nil {
			c.errorf("register.workspace: %v", err)
		} else if info, statErr := os.Stat(path); statErr != nil {
			c.warnf("register.workspace %s does not exist yet", path)
		} else if !info.IsDir() {
			c.errorf("register.workspace %s is a file", path)
		}
	}

	if recipe.Register.Skip && recipe.Register.Workspace != "" {
		c.warnf("register.skip is set, so register.workspace is never used")
	}

	remote := recipe.Register.Remote
	if remote == nil {
		return
	}
	switch {
	case strings.TrimSpace(remote.Host) == "":
		c.errorf("register.remote has no host")
	case strings.TrimSpace(remote.Group) == "":
		c.errorf("register.remote has no group")
	default:
		// A host nobody configured means the URL cannot be built at all.
		if _, _, err := folderutil.GitURLs(remote.Host, remote.Group, "check"); err != nil {
			c.errorf("register.remote: %v", err)
		}
	}
}

// checkContext is what a command line is parsed against: enough of a context
// that a `{{ .Name }}` resolves, without pretending to know the answers.
var checkContext = map[string]any{
	"Name":    "example",
	"Project": "example",
	"User":    "example",
	"Values":  map[string]any{},
}

// readsUnder reports whether anything nested under a name is read.
func readsUnder(read map[string]bool, name string) bool {
	for key := range read {
		if strings.HasPrefix(key, name+".") {
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
