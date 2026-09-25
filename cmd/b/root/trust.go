package root

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"gitlab.com/dynamo-tools/projekt/pkg/bplutil"
)

const trustLongHelp = `Allow a recipe to run commands, as it is now.

A recipe runs its ` + "`after`" + ` commands, the template it renders runs its own,
and either can call the ` + "`output`" + ` function. A recipe you wrote yourself runs
them freely. One that sits in a git repository with a remote, or renders a
template that does, runs them only once you have trusted it, and only as long
as the recipe and its template stay exactly as they were when you did.

Trusting a recipe trusts the template it renders too.

Asked on a terminal, a command that is not trusted yet asks first. Anywhere
else it is refused, and this is how to allow it beforehand.

Examples:

  b trust go-cli       # one recipe
  b trust              # every recipe of the store`

func NewBoilerplateTrustCmd(out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:               "trust [boilerplate...]",
		Short:             "Allow a recipe from a repository to run commands",
		Long:              trustLongHelp,
		ValidArgsFunction: completeRecipeNames,
		RunE: func(cmd *cobra.Command, args []string) error {
			recipes, err := recipesNamed(args)
			if err != nil {
				return err
			}
			for _, recipe := range recipes {
				if err := bplutil.Trust(recipe); err != nil {
					return err
				}
				if _, err := fmt.Fprintf(out, "Trusted %s\n", recipe.Name); err != nil {
					return err
				}
				for _, command := range recipe.After {
					if _, err := fmt.Fprintf(out, "  after: %s\n", command); err != nil {
						return err
					}
				}
			}
			return nil
		},
	}
	return cmd
}

// recipesNamed returns the recipes named, or the whole store.
func recipesNamed(names []string) ([]bplutil.Recipe, error) {
	if len(names) == 0 {
		return bplutil.List()
	}
	recipes := make([]bplutil.Recipe, 0, len(names))
	for _, name := range names {
		recipe, err := bplutil.Get(name)
		if err != nil {
			return nil, err
		}
		recipes = append(recipes, recipe)
	}
	return recipes, nil
}
