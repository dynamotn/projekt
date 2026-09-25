package bplutil

import (
	"fmt"
	"io"
	"path/filepath"

	"gitlab.com/dynamo-tools/projekt/pkg/tplutil"
)

// ApplyOptions drives `b apply`.
type ApplyOptions struct {
	// Recipes are the boilerplates to apply again. Empty means every recipe
	// the project records.
	Recipes []Recipe
	// Project is the folder to work on. Empty means the current one.
	Project string
	// Values are given on top of the recorded ones, which are replayed.
	Values tplutil.Values
	// Force rewrites the files that were edited by hand since.
	Force bool
	// Prune deletes the files the template no longer writes.
	Prune bool
	// DryRun works the whole thing out without writing.
	DryRun bool
	// Hooks runs the recipe's `after` commands again.
	//
	// Off by default: those commands ran when the project was created, and a
	// `git init` or a `gh repo create` is not something to repeat behind
	// somebody's back. A `go mod tidy` is, which is what the flag is for.
	Hooks bool
	// Log receives what is being done, or would be.
	Log io.Writer
	// In, HookOut and HookErr are the streams a hook runs with.
	In               io.Reader
	HookOut, HookErr io.Writer
}

// Applied is what one recipe's apply did.
type Applied struct {
	// Recipe is the boilerplate that was applied.
	Recipe Recipe
	// Changes are what happened to the files.
	Changes []tplutil.Change
}

// Apply renders a recipe's template over the project again and runs the
// recipe's own commands.
//
// `t apply` brings a project up to date with its *template*. A recipe is more
// than its template — it has questions of its own and commands that follow —
// so a project created by `b` is brought up to date by `b`.
func Apply(o ApplyOptions) ([]Applied, error) {
	project, err := filepath.Abs(projectOrCurrent(o.Project))
	if err != nil {
		return nil, fmt.Errorf("cannot resolve %s: %w", o.Project, err)
	}

	recipes, err := Targets(o, project)
	if err != nil {
		return nil, err
	}
	record, err := tplutil.LoadRecord(project)
	if err != nil {
		return nil, err
	}

	log := o.Log
	if log == nil {
		log = io.Discard
	}

	applied := make([]Applied, 0, len(recipes))
	for _, recipe := range recipes {
		if recipe.Source.Template == "" {
			return nil, fmt.Errorf("boilerplate %q creates from a repository, which cannot be applied again", recipe.Name)
		}
		tpl, err := tplutil.Get(recipe.Source.Template)
		if err != nil {
			return nil, fmt.Errorf("boilerplate %q: %w", recipe.Name, err)
		}

		// The recipe's recorded answers first, then anything given now.
		remembered, _ := record.FindRecipe(recipe.Name)
		values := tplutil.MergeValues(remembered.Values, o.Values)

		changes, err := tplutil.Apply(tplutil.ApplyOptions{
			Templates: []tplutil.Template{tpl},
			Dest:      project,
			Name:      remembered.Name,
			Values:    values,
			Force:     o.Force,
			Prune:     o.Prune,
			DryRun:    o.DryRun,
			In:        o.In,
			Prompt:    o.HookErr,
		})
		if err != nil {
			return nil, err
		}
		applied = append(applied, Applied{Recipe: recipe, Changes: changes})

		if !o.Hooks || len(recipe.After) == 0 {
			continue
		}
		plan := Plan{Recipe: recipe, Template: tpl, Path: project, Name: nameOr(remembered.Name, project)}
		hooks := CreateOptions{
			Recipe: recipe, Values: values, DryRun: o.DryRun,
			In: o.In, HookOut: o.HookOut, HookErr: o.HookErr,
		}
		if err := RunAfter(log, plan, values, hooks); err != nil {
			return nil, err
		}
	}
	return applied, nil
}

// Targets returns the recipes an apply covers: the ones it was given, or
// everything the project remembers being created from.
func Targets(o ApplyOptions, project string) ([]Recipe, error) {
	if len(o.Recipes) > 0 {
		return o.Recipes, nil
	}

	record, err := tplutil.LoadRecord(project)
	if err != nil {
		return nil, err
	}
	if len(record.Recipes) == 0 {
		return nil, fmt.Errorf("%s does not record being created from a boilerplate; name one, or use `t apply` for its templates", project)
	}

	recipes := make([]Recipe, 0, len(record.Recipes))
	for _, entry := range record.Recipes {
		recipe, err := Get(entry.Recipe)
		if err != nil {
			return nil, fmt.Errorf("%s was created from %q: %w", project, entry.Recipe, err)
		}
		recipes = append(recipes, recipe)
	}
	return recipes, nil
}

// PrintApplied writes what each recipe did, or would.
func PrintApplied(out io.Writer, applied []Applied, dryRun bool) error {
	for _, one := range applied {
		verb := "Applied"
		if dryRun {
			verb = "Would apply"
		}
		if _, err := fmt.Fprintf(out, "%s %s: %s\n", verb, one.Recipe.Name, tplutil.Summary(one.Changes)); err != nil {
			return err
		}
		for _, change := range one.Changes {
			if !change.Writes() {
				continue
			}
			if _, err := fmt.Fprintf(out, "  %s\t%s\n", change.Status, change.Path); err != nil {
				return err
			}
		}
	}
	return nil
}

func projectOrCurrent(project string) string {
	if project == "" {
		return "."
	}
	return project
}

func nameOr(name, project string) string {
	if name != "" {
		return name
	}
	return filepath.Base(project)
}
