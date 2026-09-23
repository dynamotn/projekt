package root

import (
	"github.com/spf13/cobra"

	"gitlab.com/dynamo.foss/projekt/pkg/bplutil"
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
