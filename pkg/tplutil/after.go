package tplutil

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"unicode"

	"gitlab.com/dynamo-tools/projekt/pkg/cli"
)

// HookOptions drives the commands a template runs once its files are there.
type HookOptions struct {
	// Template is the template whose manifest the commands come from.
	Template Template
	// Dir is where the commands run: the folder the template wrote into.
	Dir string
	// Context is what a command line is rendered with, the same data the
	// template itself was rendered with.
	Context map[string]any
	// DryRun prints the commands instead of running them.
	DryRun bool
	// Log receives what is being done, or would be. A hook that runs out of
	// sight is a hook nobody can debug.
	Log io.Writer
	// In, Out and Err are the streams a command runs with.
	In       io.Reader
	Out, Err io.Writer
}

// RunAfter runs a template's `after` commands in the folder it just wrote.
//
// Each one is rendered first, so it can use the values, and printed before it
// runs. The first failure stops the rest, because the second command usually
// assumes the first one worked.
func RunAfter(o HookOptions) error {
	manifest, err := LoadManifest(o.Template)
	if err != nil {
		return err
	}
	if len(manifest.After) == 0 {
		return nil
	}

	log := o.Log
	if log == nil {
		log = io.Discard
	}
	delims := manifest.delims()

	for i, command := range manifest.After {
		name := fmt.Sprintf("%s:after[%d]", o.Template.Name, i)
		rendered, err := executeMode(TemplateOrigin(o.Template), delims, name, command, o.Context, o.DryRun)
		if err != nil {
			return err
		}
		line := strings.TrimSpace(string(rendered))
		// A command a condition rendered away is not a command.
		if line == "" {
			continue
		}

		if o.DryRun {
			if _, err := fmt.Fprintf(log, "[DRY RUN] Would run: %s\n", line); err != nil {
				return err
			}
			continue
		}
		if _, err := fmt.Fprintf(log, "Running: %s\n", line); err != nil {
			return err
		}
		if err := o.run(line); err != nil {
			return fmt.Errorf("%s: %w", line, err)
		}
	}
	return nil
}

// run runs one command through a shell, in the folder the template wrote.
func (o HookOptions) run(command string) error {
	if err := Authorize(TemplateOrigin(o.Template), command); err != nil {
		return err
	}
	env := []string{"PROJEKT_TEMPLATE=" + o.Template.Name, "PROJEKT_PATH=" + o.Dir}
	if name, ok := o.Context["Name"].(string); ok {
		env = append(env, "PROJEKT_NAME="+name)
	}
	values, _ := o.Context["Values"].(map[string]any)
	return RunShell(command, o.Dir, append(env, ValueEnv(values)...), o.In, o.Out, o.Err)
}

// RunShell runs one hook line with sh -c in dir, with env added to the
// environment.
//
// Through a shell because a hook that cannot use a pipe or an && is not much
// of a hook, and because that is what the line looks like. Always sh, never
// the login shell: the line is POSIX, and a recipe shared between people must
// not mean something else under fish or nu.
func RunShell(command, dir string, env []string, in io.Reader, out, errOut io.Writer) error {
	cmd := cli.Command("sh", "-c", command)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), env...)
	cmd.Stdin = in
	cmd.Stdout = out
	cmd.Stderr = errOut
	return cmd.Run()
}

// ValueEnv passes each value to a hook as PROJEKT_VALUE_<NAME>, name in upper
// case with anything but letters and digits made an underscore.
//
// A value written into the line itself is shell code, and a project's record
// is text anybody could have committed; "$PROJEKT_VALUE_MODULE" is only ever
// a word. A list or a map is passed as JSON.
func ValueEnv(values Values) []string {
	names := make([]string, 0, len(values))
	for name := range values {
		names = append(names, name)
	}
	sort.Strings(names)

	env := make([]string, 0, len(names))
	for _, name := range names {
		key := strings.Map(func(r rune) rune {
			if r < unicode.MaxASCII && (unicode.IsLetter(r) || unicode.IsDigit(r)) {
				return unicode.ToUpper(r)
			}
			return '_'
		}, name)
		var text string
		switch value := values[name].(type) {
		case nil:
		case string, bool, int, int64, float64:
			text = fmt.Sprint(value)
		default:
			data, err := json.Marshal(value)
			if err != nil {
				continue
			}
			text = string(data)
		}
		env = append(env, "PROJEKT_VALUE_"+key+"="+text)
	}
	return env
}
