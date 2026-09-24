package tplutil

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
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
		rendered, err := executeWith(delims, name, command, o.Context)
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
//
// Through a shell because a hook that cannot use a pipe or an && is not much
// of a hook, and because that is what the line in the manifest looks like.
func (o HookOptions) run(command string) error {
	shell := strings.TrimSpace(os.Getenv("SHELL"))
	if shell == "" {
		shell = "/bin/sh"
	}

	cmd := exec.Command(shell, "-c", command)
	cmd.Dir = o.Dir
	cmd.Env = append(os.Environ(),
		"PROJEKT_TEMPLATE="+o.Template.Name,
		"PROJEKT_PATH="+o.Dir,
	)
	if name, ok := o.Context["Name"].(string); ok {
		cmd.Env = append(cmd.Env, "PROJEKT_NAME="+name)
	}
	cmd.Stdin = o.In
	cmd.Stdout = o.Out
	cmd.Stderr = o.Err
	return cmd.Run()
}
