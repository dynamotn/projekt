package bplutil

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"gitlab.com/dynamo-tools/projekt/pkg/folderutil"
	"gitlab.com/dynamo-tools/projekt/pkg/lazypath"
	"gitlab.com/dynamo-tools/projekt/pkg/tplutil"
)

// RunAfter runs a recipe's commands in the new project.
//
// Each one is rendered first, so it can use the values, and printed before it
// runs: a recipe that runs commands should never do so out of sight. The first
// failure stops the rest, because the second command of a recipe usually
// assumes the first one worked.
func RunAfter(out io.Writer, plan Plan, values tplutil.Values, o CreateOptions) error {
	if len(plan.Recipe.After) == 0 {
		return nil
	}

	base, err := plan.BaseContext()
	if err != nil {
		return err
	}
	base["Values"] = map[string]any(values)

	for i, command := range plan.Recipe.After {
		rendered, err := renderCommand(plan.Recipe, i, command, base)
		if err != nil {
			return err
		}
		if strings.TrimSpace(rendered) == "" {
			continue
		}

		if o.DryRun {
			if _, err := fmt.Fprintf(out, "[DRY RUN] Would run: %s\n", rendered); err != nil {
				return err
			}
			continue
		}

		if _, err := fmt.Fprintf(out, "Running: %s\n", rendered); err != nil {
			return err
		}
		if err := runCommand(plan, rendered, o); err != nil {
			return fmt.Errorf("%s: %w", rendered, err)
		}
	}

	return nil
}

// renderCommand runs one command line through the template engine.
func renderCommand(recipe Recipe, index int, command string, base map[string]any) (string, error) {
	rendered, err := tplutil.RenderString(TrustOrigin(recipe), fmt.Sprintf("%s:after[%d]", recipe.Name, index), command, base)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(rendered), nil
}

// runCommand runs one command through a shell, in the new project.
//
// Through a shell because a hook that cannot use a pipe or an && is not much
// of a hook, and because that is what the line in the recipe looks like.
func runCommand(plan Plan, command string, o CreateOptions) error {
	if err := tplutil.Authorize(TrustOrigin(plan.Recipe), command); err != nil {
		return err
	}
	shell := strings.TrimSpace(os.Getenv("SHELL"))
	if shell == "" {
		shell = "/bin/sh"
	}

	cmd := exec.Command(shell, "-c", command)
	cmd.Dir = plan.Path
	cmd.Env = append(os.Environ(),
		"PROJEKT_NAME="+plan.Name,
		"PROJEKT_PATH="+plan.Path,
		"PROJEKT_BOILERPLATE="+plan.Recipe.Name,
	)
	cmd.Stdin = o.In
	cmd.Stdout = o.HookOut
	cmd.Stderr = o.HookErr

	return cmd.Run()
}

// SetUpRemote makes the new project a repository and points it at one.
func SetUpRemote(out io.Writer, plan Plan, o CreateOptions) error {
	remote := plan.Recipe.Register.Remote
	if remote == nil {
		return nil
	}

	name := strings.TrimSpace(remote.Name)
	if name == "" {
		name = plan.Name
	}
	primary, _, err := folderutil.GitURLs(remote.Host, remote.Group, name)
	if err != nil {
		return err
	}

	if o.DryRun {
		_, err := fmt.Fprintf(out, "[DRY RUN] Would point %s at %s\n", plan.Path, primary)
		return err
	}

	if err := folderutil.InitRepo(plan.Path, remote.Branch); err != nil {
		return err
	}
	if err := folderutil.SetRemote(plan.Path, "origin", primary); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "origin is %s\n", primary); err != nil {
		return err
	}

	if !remote.InRepos {
		return nil
	}
	// Recording it under the workspace is what makes `folder sync` reproduce
	// the project on the next machine.
	workspace, covered := folderutil.CoveringWorkspace(plan.Path)
	if !covered {
		return fmt.Errorf("register.remote.inRepos: %s is not inside a workspace to record it in", plan.Path)
	}
	if err := lazypath.AddRepoToFolder(workspace.Path, name, plan.Name); err != nil {
		return err
	}
	_, err = fmt.Fprintf(out, "Recorded %s under %s\n", name, workspace.Path)
	return err
}
