package root

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"gitlab.com/dynamo.foss/projekt/pkg/bplutil"
)

func NewBoilerplateShowCmd(out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:               "show [boilerplate]",
		Short:             "Print a boilerplate recipe",
		Args:              cobra.ExactArgs(1),
		Aliases:           []string{"s", "cat"},
		ValidArgsFunction: completeRecipeNames,
		RunE: func(cmd *cobra.Command, args []string) error {
			recipe, err := bplutil.Get(args[0])
			if err != nil {
				return err
			}
			return bplutil.ShowRecipe(out, recipe)
		},
	}

	return cmd
}

func NewBoilerplatePathCmd(out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:               "path [boilerplate]",
		Short:             "Print the path of the boilerplate folder, or of one recipe",
		Args:              cobra.MaximumNArgs(1),
		Aliases:           []string{"p", "where"},
		ValidArgsFunction: completeRecipeNames,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 1 {
				recipe, err := bplutil.Get(args[0])
				if err != nil {
					return err
				}
				_, err = fmt.Fprintln(out, recipe.Path)
				return err
			}

			dir, err := bplutil.Dir()
			if err != nil {
				return err
			}
			_, err = fmt.Fprintln(out, dir)
			return err
		},
	}

	return cmd
}
