package tplutil

import (
	"bytes"
	"fmt"
	"regexp"
	"sort"
	"strings"

	yaml "go.yaml.in/yaml/v3"
)

// Replacement turns a literal that appears throughout a project into the
// template expression that will put it back.
//
// It is the tedious half of writing a template: the project already says
// `myapp` in nineteen places, and every one of them has to become
// `{{ .Name }}`.
type Replacement struct {
	// Literal is the text as it appears in the project.
	Literal string
	// Expr is the template path to put there, without the braces or the
	// leading dot: "Name", "Values.module".
	Expr string
}

// Action is what the replacement writes into the template.
func (r Replacement) Action() string {
	return "{{ ." + r.Expr + " }}"
}

// exprPattern is what a replacement may target: a field path, nothing else.
// Anything more would be a template someone wrote by accident.
var exprPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*(\.[A-Za-z_][A-Za-z0-9_]*)*$`)

// ParseReplacements reads the `--replace literal=Expr` arguments.
//
// The split is on the *last* `=`, because a literal may well contain one —
// `--replace VERSION=1.0=Values.version` — while a field path never does.
func ParseReplacements(arguments []string) ([]Replacement, error) {
	replacements := make([]Replacement, 0, len(arguments))

	for _, argument := range arguments {
		at := strings.LastIndex(argument, "=")
		if at <= 0 {
			return nil, fmt.Errorf("invalid --replace %q: expected literal=Expr, as in myapp=Name", argument)
		}
		literal, expr := argument[:at], strings.TrimPrefix(strings.TrimSpace(argument[at+1:]), ".")
		if literal == "" {
			return nil, fmt.Errorf("invalid --replace %q: the literal is empty", argument)
		}
		if !exprPattern.MatchString(expr) {
			return nil, fmt.Errorf("invalid --replace %q: %q is not a template path like Name or Values.module", argument, expr)
		}
		replacements = append(replacements, Replacement{Literal: literal, Expr: expr})
	}

	// The longest literal first, so that replacing `myapp` never eats half of
	// `example.com/myapp`.
	sort.SliceStable(replacements, func(i, j int) bool {
		return len(replacements[i].Literal) > len(replacements[j].Literal)
	})
	return replacements, nil
}

// apply rewrites one piece of text and reports how many substitutions it made.
func apply(replacements []Replacement, text string) (string, int) {
	count := 0
	for _, replacement := range replacements {
		occurrences := strings.Count(text, replacement.Literal)
		if occurrences == 0 {
			continue
		}
		text = strings.ReplaceAll(text, replacement.Literal, replacement.Action())
		count += occurrences
	}
	return text, count
}

// isText reports whether content can be rewritten safely.
//
// A NUL byte means an image, an archive or a compiled binary, and rewriting
// one would corrupt it without anybody noticing until the template is used.
func isText(content []byte) bool {
	return !bytes.Contains(content, []byte{0})
}

// manifestFor builds the `.vars.yaml` a set of replacements implies: every
// `Values.` target becomes a question, with what the project already said as
// its default.
//
// The values the template gets for free — `.Name` and the rest — are not
// questions, so they are left out.
func manifestFor(replacements []Replacement) ([]byte, error) {
	manifest := Manifest{}
	for _, replacement := range replacements {
		name := strings.TrimPrefix(replacement.Expr, "Values.")
		if name == replacement.Expr {
			continue
		}
		manifest.Vars = append(manifest.Vars, Var{Name: name, Default: replacement.Literal})
	}
	if len(manifest.Vars) == 0 {
		return nil, nil
	}

	sort.SliceStable(manifest.Vars, func(i, j int) bool { return manifest.Vars[i].Name < manifest.Vars[j].Name })

	body, err := yaml.Marshal(manifest)
	if err != nil {
		return nil, fmt.Errorf("cannot write the manifest: %w", err)
	}
	header := "# Written by `t add --replace`. The defaults are what the project\n" +
		"# it came from already said; the prompts are yours to fill in.\n"
	return append([]byte(header), body...), nil
}
