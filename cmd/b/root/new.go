package root

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"gitlab.com/dynamo.foss/projekt/pkg/bplutil"
	"gitlab.com/dynamo.foss/projekt/pkg/tplutil"
)

const newLongHelp = `Create a project from a boilerplate recipe.

A recipe lives in your boilerplate folder as a YAML file. It says where the
files come from, what to ask for, and where the finished project belongs:

  description: Go CLI with cobra and a Makefile
  source:
    template: go-cli        # a folder template of the ` + "`t`" + ` store
  vars:                     # the same shape as a template's .vars.yaml
    - name: module
      default: "example.com/{{ .Name }}"
      required: true
  register:
    workspace: ~/work       # a plain name is created in here
    prefix: work
    tags: [go, work]

Given a plain name the project is created inside the workspace, so that it can
be jumped to straight away; given a path it is created there instead.

The project is added to the projekt configuration, unless it lands inside a
workspace that already reaches it, in which case there is nothing to add.

Examples:

  b new go-cli myapp                      # ~/work/myapp, then pj work-myapp
  b new go-cli myapp -i                   # ask for what the recipe needs
  b new go-cli ./scratch/myapp            # a path is used as it is
  b new go-cli myapp --set module=example.com/myapp
  b new go-cli myapp --dry-run            # the whole plan, nothing written`

func NewBoilerplateNewCmd(out io.Writer) *cobra.Command {
	var (
		sets        []string
		valueFiles  []string
		interactive bool
	)
	o := bplutil.CreateOptions{}

	cmd := &cobra.Command{
		Use:               "new [boilerplate] [name or path]",
		Short:             "Create a project from a boilerplate",
		Long:              newLongHelp,
		Args:              cobra.ExactArgs(2),
		Aliases:           []string{"n", "create", "init"},
		ValidArgsFunction: completeRecipeNames,
		RunE: func(cmd *cobra.Command, args []string) error {
			recipe, err := bplutil.Get(args[0])
			if err != nil {
				return err
			}
			o.Recipe = recipe
			o.Target = args[1]
			// The rendered content of a dry run is a lot of text nobody asked
			// for; what it would do is the point, and that goes to the caller.
			o.Out = io.Discard
			o.Log = out
			// A hook is a command someone will want to watch run.
			o.In, o.HookOut, o.HookErr = cmd.InOrStdin(), out, cmd.ErrOrStderr()

			fileValues, err := tplutil.LoadValuesFiles(valueFiles)
			if err != nil {
				return err
			}
			setValues, err := tplutil.ParseSet(sets)
			if err != nil {
				return err
			}
			o.Values = tplutil.MergeValues(fileValues, setValues)

			if interactive {
				if o.Values, err = askForValues(cmd, o); err != nil {
					return err
				}
			}

			result, err := bplutil.Create(o)
			if err != nil {
				return err
			}
			return bplutil.PrintPlan(out, result.Plan, result.Files, o.DryRun)
		},
	}

	f := cmd.Flags()
	f.StringVarP(&o.Workspace, "workspace", "w", "", "Folder the project is created in, overriding the recipe's")
	f.StringArrayVarP(&sets, "set", "s", nil, "Set a value, like -s key=value or -s author.name=me (repeatable)")
	f.StringArrayVarP(&valueFiles, "values", "f", nil, "YAML file of values (repeatable)")
	f.BoolVarP(&interactive, "interactive", "i", false, "Ask for the values the boilerplate needs")
	f.BoolVar(&o.NoHooks, "no-hooks", false, "Don't run the recipe's `after` commands")
	f.BoolVarP(&o.NoRegister, "no-register", "", false, "Don't add the project to the projekt config")
	f.BoolVarP(&o.Force, "force", "F", false, "Create into a folder that is not empty, overwriting files")
	f.BoolVarP(&o.DryRun, "dry-run", "d", false, "Print the plan instead of creating anything")

	return cmd
}

// askForValues asks for whatever the recipe needs and is not already set.
//
// The questions go to stderr, so that the plan printed on stdout stays
// machine-readable.
func askForValues(cmd *cobra.Command, o bplutil.CreateOptions) (tplutil.Values, error) {
	plan, err := bplutil.Resolve(o)
	if err != nil {
		return nil, err
	}

	vars, err := plan.Vars()
	if err != nil {
		return nil, err
	}
	base, err := plan.BaseContext()
	if err != nil {
		return nil, err
	}

	prompt := cmd.ErrOrStderr()
	if len(vars) == 0 {
		fmt.Fprintf(prompt, "%s takes no values.\n", o.Recipe.Name)
		return o.Values, nil
	}
	fmt.Fprintf(prompt, "Values for %s:\n", o.Recipe.Name)

	values, err := tplutil.Prompter{In: cmd.InOrStdin(), Out: prompt}.Ask(vars, o.Values, base)
	if err != nil {
		return nil, err
	}
	fmt.Fprintln(prompt)
	return values, nil
}
