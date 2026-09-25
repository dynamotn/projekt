package root

import (
	"github.com/spf13/cobra"

	t "gitlab.com/dynamo.foss/projekt/cmd/t/root"
	"gitlab.com/dynamo.foss/projekt/pkg/bplutil"
	"gitlab.com/dynamo.foss/projekt/pkg/tplutil"
)

// completeRecipeNames completes the first argument with the recipes of the
// store, so `b new <TAB>` offers what can actually be created.
func completeRecipeNames(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveDefault
	}

	recipes, err := bplutil.List()
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}

	names := make([]string, 0, len(recipes))
	for _, recipe := range recipes {
		name := recipe.Name
		if recipe.Description != "" {
			name += "\t" + recipe.Description
		}
		names = append(names, name)
	}
	return names, cobra.ShellCompDirectiveNoFileComp
}

// completeMoreRecipeNames completes a variadic recipe argument, leaving out
// the ones already named.
func completeMoreRecipeNames(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	recipes, err := bplutil.List()
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}

	named := map[string]bool{}
	for _, name := range args {
		named[name] = true
	}

	names := make([]string, 0, len(recipes))
	for _, recipe := range recipes {
		if named[recipe.Name] {
			continue
		}
		names = append(names, recipe.Name+"\t"+recipe.Description)
	}
	return names, cobra.ShellCompDirectiveNoFileComp
}

// completeSetKeys completes `--set` with what the recipes in play ask for: a
// recipe's own questions, or the ones its template reads.
func completeSetKeys(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	recipes, err := completionRecipes(cmd, args)
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}

	var vars []tplutil.Var
	for _, recipe := range recipes {
		vars = append(vars, recipeVars(recipe)...)
	}
	return t.CompleteValueAssignment(vars, toComplete)
}

// completionRecipes works out which recipes a completion is about: the ones
// named on the command line, or everything the project records.
func completionRecipes(cmd *cobra.Command, args []string) ([]bplutil.Recipe, error) {
	if len(args) > 0 {
		recipes := make([]bplutil.Recipe, 0, len(args))
		for _, name := range args {
			recipe, err := bplutil.Get(name)
			if err != nil {
				return nil, err
			}
			recipes = append(recipes, recipe)
		}
		return recipes, nil
	}

	project, err := cmd.Flags().GetString("project")
	if err != nil {
		// `b new` has no --project: there is no project yet to ask.
		return nil, nil
	}
	if project == "" {
		project = "."
	}
	return bplutil.Targets(bplutil.ApplyOptions{}, project)
}

// recipeVars is what a recipe asks for: its own list, or the template's.
func recipeVars(recipe bplutil.Recipe) []tplutil.Var {
	if len(recipe.Vars) > 0 {
		return recipe.Vars
	}
	if recipe.Source.Template == "" {
		return nil
	}
	tpl, err := tplutil.Get(recipe.Source.Template)
	if err != nil {
		return nil
	}
	vars, err := tplutil.Vars(tpl)
	if err != nil {
		return nil
	}
	return vars
}

// completeDirs offers folders alone, for the flags that name a project.
func completeDirs(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
	return nil, cobra.ShellCompDirectiveFilterDirs
}

// registerValueCompletion wires the flags whose values are worth completing.
func registerValueCompletion(cmd *cobra.Command) {
	if cmd.Flag("set") != nil {
		_ = cmd.RegisterFlagCompletionFunc("set", completeSetKeys)
	}
	for _, name := range []string{"project", "workspace"} {
		if cmd.Flag(name) != nil {
			_ = cmd.RegisterFlagCompletionFunc(name, completeDirs)
		}
	}
}
