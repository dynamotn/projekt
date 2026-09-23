package tplutil

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// Prompter asks for the values a template needs.
//
// Questions go to Out, which is stderr for the command, so that `t new --dry-run
// --interactive > file` still writes only the rendered template to the file.
type Prompter struct {
	In  io.Reader
	Out io.Writer
}

// Ask walks the variables and returns the values, the ones already given left
// untouched: a value passed with --set is an answer, and is not asked again.
//
// Reading stops at end of input rather than failing, so answers can be piped
// in and the rest fall back to their defaults. A required variable with no
// default and no answer is an error, because rendering it would be wrong.
func (p Prompter) Ask(vars []Var, given Values, base map[string]any) (Values, error) {
	values := MergeValues(given, nil)
	if len(vars) == 0 {
		return values, nil
	}

	reader := bufio.NewReader(p.In)
	eof := false

	for _, v := range vars {
		if _, ok := lookup(values, v.Name); ok {
			continue
		}

		fallback, err := p.renderDefault(v, base)
		if err != nil {
			return nil, err
		}

		for attempt := 0; ; attempt++ {
			answer := fallback
			if !eof {
				typed, err := p.ask(reader, v, fallback)
				if errors.Is(err, io.EOF) {
					eof = true
				} else if err != nil {
					return nil, err
				} else if typed != "" {
					answer = typed
				}
			}

			if answer == "" {
				if !v.Required {
					break
				}
				if eof {
					return nil, fmt.Errorf("%s is required and there is nothing left to read", v.Name)
				}
				fmt.Fprintf(p.Out, "    %s is required.\n", v.Name)
				continue
			}

			parsed, err := parseAnswer(v, answer)
			if err != nil {
				if eof {
					return nil, fmt.Errorf("%s: %w", v.Name, err)
				}
				fmt.Fprintf(p.Out, "    %v\n", err)
				continue
			}
			if err := assign(values, strings.Split(v.Name, "."), parsed); err != nil {
				return nil, fmt.Errorf("%s: %w", v.Name, err)
			}
			break
		}
	}

	return values, nil
}

// ask writes one question and reads one line.
func (p Prompter) ask(reader *bufio.Reader, v Var, fallback string) (string, error) {
	question := "  " + v.question()
	if v.Type == VarChoice {
		question += " (" + strings.Join(v.Choices, "/") + ")"
	}
	if fallback != "" {
		question += " [" + fallback + "]"
	}
	if _, err := fmt.Fprint(p.Out, question, ": "); err != nil {
		return "", err
	}

	line, err := reader.ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", err
	}
	if errors.Is(err, io.EOF) {
		// A last line without a newline is still an answer; an empty one means
		// there is nothing more to read.
		if strings.TrimSpace(line) == "" {
			fmt.Fprintln(p.Out)
			return "", io.EOF
		}
	}
	return strings.TrimSpace(line), nil
}

// renderDefault runs the default through the engine, so that a manifest can
// say `default: "{{ .User }}"` or `default: "{{ .Name }}-api"`.
func (p Prompter) renderDefault(v Var, base map[string]any) (string, error) {
	if v.Default == "" || !strings.Contains(v.Default, "{{") {
		return v.Default, nil
	}

	rendered, err := execute("default:"+v.Name, v.Default, base)
	if err != nil {
		return "", fmt.Errorf("default of %s: %w", v.Name, err)
	}
	return strings.TrimSpace(string(rendered)), nil
}

// parseAnswer turns what was typed into the type the variable declares.
func parseAnswer(v Var, answer string) (any, error) {
	switch v.Type {
	case VarInt:
		n, err := strconv.Atoi(answer)
		if err != nil {
			return nil, fmt.Errorf("%q is not a whole number", answer)
		}
		return n, nil
	case VarBool:
		switch strings.ToLower(answer) {
		case "y", "yes", "true", "1":
			return true, nil
		case "n", "no", "false", "0":
			return false, nil
		default:
			return nil, fmt.Errorf("%q is not a yes or a no", answer)
		}
	case VarChoice:
		for _, choice := range v.Choices {
			if strings.EqualFold(answer, choice) {
				return choice, nil
			}
		}
		return nil, fmt.Errorf("%q is not one of: %s", answer, strings.Join(v.Choices, ", "))
	case VarList:
		parts := strings.Split(answer, ",")
		list := make([]string, 0, len(parts))
		for _, part := range parts {
			if trimmed := strings.TrimSpace(part); trimmed != "" {
				list = append(list, trimmed)
			}
		}
		return list, nil
	default:
		return answer, nil
	}
}

// lookup follows a dotted key through the values.
func lookup(values Values, name string) (any, bool) {
	var current any = map[string]any(values)
	for _, key := range strings.Split(name, ".") {
		asMap, ok := toMap(current)
		if !ok {
			return nil, false
		}
		current, ok = asMap[key]
		if !ok {
			return nil, false
		}
	}
	return current, true
}
