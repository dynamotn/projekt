package tplutil

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"unicode"

	"gitlab.com/dynamo-tools/projekt/pkg/aiutil"
)

// SuggestOptions say how SuggestReplacements talks to the AI command and
// where it reports.
type SuggestOptions struct {
	ShowPrompt bool // print the prompt and stop
	// Given are the replacements typed with --replace: a suggested key that
	// one of them already uses would make a second value of that name.
	Given    []Replacement
	Out, Err io.Writer
	// Ask is the AI call; nil means aiutil.Ask. Tests set it.
	Ask func(prompt string, errOut io.Writer) (string, error)
}

const (
	maxLiteral = 200
	minLiteral = 3 // a shorter one rewrites far more than it names
)

var actionPattern = regexp.MustCompile(`(?s)\{\{.*?\}\}`)

var keyPattern = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// SuggestReplacements asks the AI which literals of source should become
// values, keeps only the pairs that hold, prints them with the equivalent
// `t add … --replace` line, and returns them. It adds nothing.
//
// The answer is only ever text: it is parsed and checked, never run.
func SuggestReplacements(source string, o SuggestOptions) ([]Replacement, error) {
	prompt, found, outside, err := suggestPrompt(source)
	if err != nil {
		if _, lookErr := exec.LookPath("git"); lookErr != nil {
			return nil, fmt.Errorf("--suggest needs git to tell which files may be sent: %w", err)
		}
		return nil, err
	}
	if o.ShowPrompt {
		_, err := fmt.Fprint(o.Out, prompt)
		return nil, err
	}

	ask := o.Ask
	if ask == nil {
		ask = aiutil.Ask
	}
	answer, err := ask(prompt, o.Err)
	if err != nil {
		return nil, err
	}

	var kept []Replacement
	taken, keys := map[string]bool{}, map[string]bool{}
	for _, r := range o.Given {
		if key, ok := strings.CutPrefix(r.Expr, "Values."); ok {
			keys[key] = true
		}
	}
	var writeErr error // the first failed write; the loop goes on, the caller hears of it
	note := func(_ int, err error) {
		if writeErr == nil {
			writeErr = err
		}
	}
	drop := func(line, why string) { note(fmt.Fprintf(o.Err, "dropped %q: %s\n", line, why)) }
	for _, line := range strings.Split(answer, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		at := strings.LastIndex(line, ": ")
		if at < 0 {
			drop(line, "not in the form literal: key")
			continue
		}
		literal, key := line[:at], strings.TrimSpace(line[at+2:])
		switch {
		case !keyPattern.MatchString(key):
			drop(line, "the key is not an identifier like module or author")
		case literal == "" || len(literal) > maxLiteral:
			drop(line, fmt.Sprintf("the literal is empty or over %d bytes", maxLiteral))
		case strings.IndexFunc(literal, unicode.IsControl) >= 0:
			drop(line, "the literal holds a control character")
		case strings.Contains(literal, "{{") || strings.Contains(literal, "}}"):
			drop(line, "the literal is template syntax")
		case len(literal) < minLiteral:
			drop(line, fmt.Sprintf("the literal is shorter than %d bytes", minLiteral))
		case !found(literal):
			drop(line, "the literal was not found in the files that may be sent")
		case !outside(literal):
			drop(line, "the literal occurs only inside template actions: template syntax")
		case taken[literal]:
			drop(line, "the literal came earlier")
		case keys[key]:
			drop(line, "the key came earlier")
		default:
			taken[literal], keys[key] = true, true
			kept = append(kept, Replacement{Literal: literal, Expr: "Values." + key})
		}
	}

	command := "t add " + shellQuote(source)
	for _, r := range kept {
		note(fmt.Fprintf(o.Out, "%s -> %s\n", r.Literal, r.Expr))
		command += " --replace " + shellQuote(r.Literal+"="+r.Expr)
	}
	if writeErr != nil {
		return nil, writeErr
	}
	if len(kept) == 0 {
		_, err := fmt.Fprintln(o.Err, "no suggestion held")
		return nil, err
	}
	if _, err := fmt.Fprintln(o.Out, command); err != nil {
		return nil, err
	}
	return kept, nil
}

// suggestPrompt builds the prompt and a test of whether a literal occurs in
// what the prompt may quote: the files Files let through or a path of Tree.
func suggestPrompt(source string) (string, func(string) bool, func(string) bool, error) {
	project, names, tree := source, []string{"go.mod", "package.json", "pyproject.toml", "Cargo.toml", "README.md"}, []string(nil)
	if info, err := os.Stat(source); err == nil && !info.IsDir() {
		project, names, tree = filepath.Dir(source), []string{filepath.Base(source)}, []string{filepath.Base(source)}
	} else {
		if tree, err = aiutil.Tree(source, 200, 0); err != nil {
			return "", nil, nil, err
		}
	}
	files, err := aiutil.Files(project, names...)
	if err != nil {
		return "", nil, nil, err
	}

	var b strings.Builder
	b.WriteString("Which text in this software project is specific to it and should become a template value? " +
		"A literal is text that occurs in the files below and belongs to this project: its name, module path, author, URLs. " +
		"A key is a short lower-case identifier for it, like name, module or author. " +
		"Answer with lines of the form `literal: key`, one per line, nothing else.\n\n")
	fmt.Fprintf(&b, "Files:\n%s\n", strings.Join(tree, "\n"))
	for _, f := range files {
		fmt.Fprintf(&b, "\n%s:\n%s\n", f.Path, f.Content)
	}

	inPaths := func(literal string) bool {
		return slices.ContainsFunc(tree, func(p string) bool { return strings.Contains(p, literal) })
	}
	found := func(literal string) bool {
		return inPaths(literal) || slices.ContainsFunc(files, func(f aiutil.File) bool { return strings.Contains(f.Content, literal) })
	}
	// Whether it occurs at least once outside a {{ … }} action; what happens
	// inside one is the replacement code's to decide.
	outside := func(literal string) bool {
		return inPaths(literal) || slices.ContainsFunc(files, func(f aiutil.File) bool {
			return strings.Contains(actionPattern.ReplaceAllString(f.Content, " "), literal)
		})
	}
	return b.String(), found, outside, nil
}

// shellQuote quotes one argument for a POSIX shell.
func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
