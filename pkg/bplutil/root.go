package bplutil

import (
	"fmt"
	"io"
	"os"

	"gitlab.com/dynamo.foss/projekt/pkg/cli"
)

// ListOption contains options for listing boilerplates.
type ListOption struct {
	// NamesOnly keeps the name column alone, the listing a script wants.
	NamesOnly bool
	cli.ListOutputOption
}

// ListRecipes displays the boilerplate store in the requested format.
func ListRecipes(out io.Writer, o *ListOption) error {
	recipes, err := List()
	if err != nil {
		return err
	}

	if len(recipes) == 0 && o.Output != cli.OutputJSON {
		dir, err := Dir()
		if err != nil {
			return err
		}
		cli.Warn("No boilerplate found in %s", dir)
		return nil
	}

	view := cli.ListView{Columns: []cli.ListColumn{
		{Header: "NAME", Key: "name"},
		{Header: "DESCRIPTION", Key: "description"},
		{Header: "CREATES FROM", Key: "source"},
		{Header: "PATH", Key: "path"},
	}}
	if o.NamesOnly {
		view.Columns = view.Columns[:1]
	}
	for _, recipe := range recipes {
		if o.NamesOnly {
			view.AppendRow(recipe.Name)
			continue
		}
		view.AppendRow(recipe.Name, recipe.Description, recipe.Origin(), recipe.Path)
	}

	return cli.EncodeList(out, view, o.ListOutputOption)
}

// ShowRecipe prints a recipe as it is written.
func ShowRecipe(out io.Writer, recipe Recipe) error {
	data, err := os.ReadFile(recipe.Path)
	if err != nil {
		return fmt.Errorf("cannot read %s: %w", recipe.Path, err)
	}
	_, err = out.Write(data)
	return err
}

// PrintPlan describes what a create is about to do, or has just done.
func PrintPlan(out io.Writer, plan Plan, files []string, dryRun bool) error {
	verb := "Created"
	if dryRun {
		verb = "Would create"
	}

	if _, err := fmt.Fprintf(out, "%s %s from %s\n", verb, plan.Path, plan.Recipe.Origin()); err != nil {
		return err
	}
	for _, file := range files {
		if _, err := fmt.Fprintf(out, "  %s\n", file); err != nil {
			return err
		}
	}

	switch {
	case plan.Register && dryRun:
		_, err := fmt.Fprintf(out, "Would register it as %q, reachable with `pj %s`\n", plan.ShortName(), plan.ShortName())
		return err
	case plan.Register:
		_, err := fmt.Fprintf(out, "Registered as %q, reachable with `pj %s`\n", plan.ShortName(), plan.ShortName())
		return err
	case plan.CoveredBy != "":
		// Nothing to add, and that is the good case: say why, and say the
		// name it already answers to.
		_, err := fmt.Fprintf(out, "Inside the workspace %s already, reachable with `pj %s`\n", plan.CoveredBy, plan.ShortName())
		return err
	default:
		_, err := fmt.Fprintf(out, "Not registered: %s\n", plan.Reason)
		return err
	}
}
