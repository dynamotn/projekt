package root

import (
	"io"

	"github.com/spf13/cobra"

	"gitlab.com/dynamo.foss/projekt/pkg/bplutil"
	"gitlab.com/dynamo.foss/projekt/pkg/tplutil"
)

const applyLongHelp = `Create a project from a boilerplate again.

` + "`t apply`" + ` brings a project up to date with its *template*. A recipe is more
than the template it renders — it has questions of its own and commands that
follow it — so a project created by ` + "`b`" + ` is brought up to date by ` + "`b`" + `.

Named no recipe, it applies every recipe the project records, in
.projekt/template.yaml. The recorded answers are replayed; --set and --values
change one and keep the rest.

Nothing edited by hand is rewritten without --force, and nothing the template
no longer writes is deleted without --prune, exactly as with ` + "`t apply`" + `.
The project is not registered again: it is already in your configuration, and
the recipe's ` + "`after`" + ` commands are not run again unless --hooks says
to: they ran when the project was created, and a ` + "`git init`" + ` is not
something to repeat.

Examples:

  b apply                        # every recipe this project records
  b apply -C ~/work/myapp        # somewhere else
  b apply go-cli --set ci=true   # one recipe, one answer changed
  b apply --dry-run              # the whole plan, nothing written
  b apply --hooks                # run the recipe's after commands again too`

func NewBoilerplateApplyCmd(out io.Writer) *cobra.Command {
	var (
		sets       []string
		valueFiles []string
	)
	o := bplutil.ApplyOptions{}

	cmd := &cobra.Command{
		Use:               "apply [boilerplate...]",
		Short:             "Create a project from a boilerplate again",
		Long:              applyLongHelp,
		Args:              cobra.ArbitraryArgs,
		Aliases:           []string{"up", "update"},
		ValidArgsFunction: completeRecipeNames,
		RunE: func(cmd *cobra.Command, args []string) error {
			for _, name := range args {
				recipe, err := bplutil.Get(name)
				if err != nil {
					return err
				}
				o.Recipes = append(o.Recipes, recipe)
			}

			fileValues, err := tplutil.LoadValuesFiles(valueFiles)
			if err != nil {
				return err
			}
			setValues, err := tplutil.ParseSet(sets)
			if err != nil {
				return err
			}
			o.Values = tplutil.MergeValues(fileValues, setValues)
			o.Log = out
			o.In, o.HookOut, o.HookErr = cmd.InOrStdin(), out, cmd.ErrOrStderr()

			applied, err := bplutil.Apply(o)
			if err != nil {
				return err
			}
			return bplutil.PrintApplied(out, applied, o.DryRun)
		},
	}

	f := cmd.Flags()
	f.StringVarP(&o.Project, "project", "C", "", "Project to work on (default is the current folder)")
	f.StringArrayVarP(&sets, "set", "s", nil, "Set a value, like -s key=value (repeatable)")
	f.StringArrayVarP(&valueFiles, "values", "f", nil, "YAML file of values (repeatable)")
	f.BoolVarP(&o.Force, "force", "F", false, "Rewrite the files that were edited by hand too")
	f.BoolVar(&o.Prune, "prune", false, "Delete the files the template no longer writes")
	f.BoolVar(&o.Hooks, "hooks", false, "Run the recipe's `after` commands again as well")
	f.BoolVarP(&o.DryRun, "dry-run", "d", false, "Print the plan instead of changing anything")

	return cmd
}
