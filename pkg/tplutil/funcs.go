package tplutil

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"strings"
	"text/template"

	yaml "go.yaml.in/yaml/v3"
)

// funcs returns the functions a template gets on top of the sprig set.
//
// They are the ones a scaffolding template keeps needing and text/template has
// no answer for: reading a file of the template, rendering a shared partial
// into a string, asking a question, and looking at the machine it runs on.
func (e *engine) funcs() template.FuncMap {
	return template.FuncMap{
		"include":         e.include,
		"includeTemplate": e.includeTemplate,
		"output":          e.output,
		"lookPath":        lookPath,
		"stat":            stat,
		"joinPath":        path.Join,
		"toYaml":          toYaml,
		"fromYaml":        fromYaml,
		"promptString":    e.promptString,
		"promptInt":       e.promptInt,
		"promptBool":      e.promptBool,
		"promptChoice":    e.promptChoice,
	}
}

// include returns a file of the template as it is, without rendering it.
//
// It is how a template keeps a long literal — a licence header, a lock file —
// out of the file that uses it.
func (e *engine) include(rel string) (string, error) {
	target, err := insideSource(e.source, rel)
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(target)
	if err != nil {
		return "", fmt.Errorf("include %q: %w", rel, err)
	}
	return string(data), nil
}

// includeTemplate renders a shared partial and returns the result, so it can
// be piped — `{{ includeTemplate "header" . | indent 4 }}`.
func (e *engine) includeTemplate(name string, data ...any) (string, error) {
	var context any
	if len(data) > 0 {
		context = data[0]
	}
	partial := e.root.Lookup(name)
	if partial == nil {
		return "", fmt.Errorf("no shared template %q in %s", name, PartialsDir)
	}
	var out strings.Builder
	if err := partial.Execute(&out, context); err != nil {
		return "", fmt.Errorf("cannot render shared template %s: %w", name, err)
	}
	return out.String(), nil
}

// output runs a command and returns its standard output, with the trailing
// newline kept so `{{ output "git" "config" "user.name" | trim }}` reads the
// way the shell does.
func (e *engine) output(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("output %s: %w", name, err)
	}
	return string(out), nil
}

// lookPath returns where an executable is, or an empty string when it is not
// installed, so a template can render what the machine can actually run.
func lookPath(name string) string {
	found, err := exec.LookPath(name)
	if err != nil {
		return ""
	}
	return found
}

// stat describes a path, or returns nil when there is nothing there, which
// makes `{{ if stat "go.mod" }}` the way to ask whether a file exists.
func stat(target string) any {
	info, err := os.Stat(target)
	if err != nil {
		return nil
	}
	return map[string]any{
		"name":    info.Name(),
		"size":    info.Size(),
		"mode":    int64(info.Mode().Perm()),
		"isDir":   info.IsDir(),
		"modTime": info.ModTime(),
	}
}

// toYaml renders a value as YAML, the way a config file wants it.
func toYaml(value any) (string, error) {
	data, err := yaml.Marshal(value)
	if err != nil {
		return "", fmt.Errorf("toYaml: %w", err)
	}
	return strings.TrimRight(string(data), "\n"), nil
}

// fromYaml reads YAML back into something a template can walk.
func fromYaml(text string) (any, error) {
	var parsed any
	if err := yaml.Unmarshal([]byte(text), &parsed); err != nil {
		return nil, fmt.Errorf("fromYaml: %w", err)
	}
	return parsed, nil
}

// promptString asks a question while rendering and remembers the answer, so
// the same question in another file is not asked twice.
//
// Without --interactive the default is taken; a question with no default is an
// error then, because rendering it would silently write an empty value.
func (e *engine) promptString(question string, fallback ...string) (string, error) {
	answer, err := e.prompt(question, Var{Name: question, Type: VarString}, defaultOf(fallback))
	if err != nil {
		return "", err
	}
	text, _ := answer.(string)
	return text, nil
}

// promptInt asks for a whole number.
func (e *engine) promptInt(question string, fallback ...any) (int, error) {
	answer, err := e.prompt(question, Var{Name: question, Type: VarInt}, scalarDefault(fallback))
	if err != nil {
		return 0, err
	}
	number, _ := answer.(int)
	return number, nil
}

// promptBool asks a yes or no question.
func (e *engine) promptBool(question string, fallback ...any) (bool, error) {
	answer, err := e.prompt(question, Var{Name: question, Type: VarBool}, scalarDefault(fallback))
	if err != nil {
		return false, err
	}
	yes, _ := answer.(bool)
	return yes, nil
}

// promptChoice asks for one of a list.
//
// The list is taken as it comes: a `list` built by sprig, or a list read out
// of a data file, is a list of anything as far as the engine is concerned.
func (e *engine) promptChoice(question string, choices any, fallback ...string) (string, error) {
	answer, err := e.prompt(question, Var{Name: question, Type: VarChoice, Choices: toStrings(choices)}, defaultOf(fallback))
	if err != nil {
		return "", err
	}
	choice, _ := answer.(string)
	return choice, nil
}

// toStrings reads a template's idea of a list back as one.
func toStrings(value any) []string {
	switch typed := value.(type) {
	case []string:
		return typed
	case []any:
		list := make([]string, 0, len(typed))
		for _, item := range typed {
			list = append(list, fmt.Sprintf("%v", item))
		}
		return list
	case nil:
		return nil
	default:
		return []string{fmt.Sprintf("%v", typed)}
	}
}

// defaultOf returns the optional default of a prompt function.
func defaultOf(fallback []string) string {
	if len(fallback) == 0 {
		return ""
	}
	return fallback[0]
}

// scalarDefault accepts the default of a typed prompt as the number or the
// boolean it is, as well as the string spelling of it.
func scalarDefault(fallback []any) string {
	if len(fallback) == 0 || fallback[0] == nil {
		return ""
	}
	return fmt.Sprintf("%v", fallback[0])
}

// zeroOf is what an unanswered question renders as during a check.
func zeroOf(v Var) any {
	switch v.Type {
	case VarInt:
		return 0
	case VarBool:
		return false
	case VarList:
		return []string(nil)
	default:
		return ""
	}
}

// prompt is what every prompt function goes through: the remembered answer,
// then the terminal, then the default.
func (e *engine) prompt(key string, v Var, fallback string) (any, error) {
	if answer, ok := e.answers[key]; ok {
		return answer, nil
	}

	if !e.ask {
		if fallback == "" {
			if e.lenient {
				// A check is asking whether the template holds together, not
				// what the answer would be.
				return zeroOf(v), nil
			}
			return nil, fmt.Errorf("%q needs an answer: run with --interactive, or give it a default", key)
		}
		answer, err := parseAnswer(v, fallback)
		if err != nil {
			return nil, fmt.Errorf("default of %q: %w", key, err)
		}
		e.answers[key] = answer
		return answer, nil
	}

	answer, err := e.read(v, fallback)
	if err != nil {
		return nil, err
	}
	e.answers[key] = answer
	return answer, nil
}

// read asks one question, again when the answer is of the wrong shape.
//
// Once the input runs out there is nobody left to ask again, so the default
// has to do, and a default that does not parse is reported rather than asked
// about forever.
func (e *engine) read(v Var, fallback string) (any, error) {
	prompter := Prompter{Out: e.out}
	for {
		typed, err := prompter.ask(e.in, v, fallback)
		eof := errors.Is(err, io.EOF)
		switch {
		case eof:
			typed = ""
		case err != nil:
			return nil, err
		}
		if typed == "" {
			typed = fallback
		}
		if typed == "" {
			return parseAnswer(v, "")
		}

		answer, err := parseAnswer(v, typed)
		if err == nil {
			return answer, nil
		}
		if eof {
			return nil, fmt.Errorf("%s: %w", v.Name, err)
		}
		fmt.Fprintf(e.out, "    %v\n", err)
	}
}
