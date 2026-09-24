package root

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"gitlab.com/dynamo.foss/projekt/pkg/tplutil"
)

const newLongHelp = `Render a Go template from your template folder.

A template is either a single file or a whole folder of files. Both the file
contents and, for a folder template, every path segment go through
text/template with the sprig function set, so a template can name the files it
creates.

Variables available to a template:

  .Name      Output name, from --name or the destination file name
  .Project   Folder the files are written to, or .Name for a folder template
  .Dir       Absolute destination folder
  .Path      Absolute path of the file being rendered
  .Template  Name of the template
  .Source    Folder the template itself lives in
  .Store     Template folder the store reads from
  .User      Current user name, .Home its home folder
  .Hostname  Name of this machine, .OS and .Arch what it runs
  .Env       The environment, as .Env.EDITOR
  .Now       Current time, .Date (2006-01-02) and .Year
  .Values    The data files, --values and --set, in that order

Besides the sprig functions a template can call include, includeTemplate,
output, lookPath, stat, joinPath, toYaml, fromYaml and the prompt functions
promptString, promptInt, promptBool and promptChoice.

A folder template may also carry a .data.yaml of default values, a .ignore
listing what not to write, and a .templates folder of shared pieces; a name
may start with executable_, private_, readonly_, symlink_, dot_ or literal_.

With --interactive the missing values are asked for, one question per value.
A template says what to ask in its .vars.yaml; without one, the questions are
the .Values keys read out of the template itself. Anything already given with
--set or --values is never asked again.

Examples:

  t new license LICENSE --set author='Jane Doe'
  t new go-cli ./myapp --set module=example.com/myapp
  t new invoice ./INV-001.md --interactive
  t new dockerfile --dry-run
  t new go-cli ./myapp --no-hooks`

func NewTemplateNewCmd(out io.Writer) *cobra.Command {
	var (
		sets        []string
		valueFiles  []string
		interactive bool
		noHooks     bool
	)
	o := tplutil.RenderOptions{Out: out}

	cmd := &cobra.Command{
		Use:               "new [template] [destination]",
		Short:             "Create files from a template",
		Long:              newLongHelp,
		Args:              cobra.RangeArgs(1, 2),
		Aliases:           []string{"n", "create", "gen"},
		ValidArgsFunction: completeTemplateNames,
		RunE: func(cmd *cobra.Command, args []string) error {
			tpl, err := tplutil.Get(args[0])
			if err != nil {
				return err
			}
			o.Template = tpl
			if len(args) > 1 {
				o.Dest = args[1]
			}

			fileValues, err := tplutil.LoadValuesFiles(valueFiles)
			if err != nil {
				return err
			}
			setValues, err := tplutil.ParseSet(sets)
			if err != nil {
				return err
			}
			// --set wins over --values, the same way a flag wins over a file.
			o.Values = tplutil.MergeValues(fileValues, setValues)
			// A data file sits under both, so what it holds is never asked
			// for again.
			if o.Values, err = tplutil.WithData(o); err != nil {
				return err
			}

			o.In = cmd.InOrStdin()
			o.Prompt = cmd.ErrOrStderr()
			o.Interactive = interactive
			if interactive {
				if o.Values, err = askForValues(cmd, o); err != nil {
					return err
				}
			}

			written, err := tplutil.Render(o)
			if err != nil {
				return err
			}
			if !o.DryRun {
				for _, path := range written {
					if _, err := fmt.Fprintln(out, path); err != nil {
						return err
					}
				}
			}
			if noHooks {
				return nil
			}
			return runHooks(cmd, out, o)
		},
	}

	f := cmd.Flags()
	f.StringVarP(&o.Name, "name", "n", "", "Name of the rendered file, also available as '.Name'")
	f.StringArrayVarP(&sets, "set", "s", nil, "Set a template value, like -s key=value or -s author.name=me (repeatable)")
	f.StringArrayVarP(&valueFiles, "values", "f", nil, "YAML file of template values (repeatable)")
	f.BoolVarP(&interactive, "interactive", "i", false, "Ask for the values the template needs")
	f.BoolVarP(&o.Force, "force", "F", false, "Overwrite files that already exist")
	f.BoolVarP(&o.DryRun, "dry-run", "d", false, "Print the rendered result instead of writing files")
	f.BoolVar(&noHooks, "no-hooks", false, "Don't run the template's `after` commands")

	return cmd
}

// runHooks runs the template's `after` commands in the folder it wrote.
//
// What is being done goes to the caller's stream, and so does the output of
// the commands: a hook is something someone will want to watch run.
func runHooks(cmd *cobra.Command, out io.Writer, o tplutil.RenderOptions) error {
	dir, err := tplutil.Destination(o)
	if err != nil {
		return err
	}
	base, err := tplutil.BaseContext(o)
	if err != nil {
		return err
	}

	return tplutil.RunAfter(tplutil.HookOptions{
		Template: o.Template,
		Dir:      dir,
		Context:  base,
		DryRun:   o.DryRun,
		Log:      out,
		In:       cmd.InOrStdin(),
		Out:      out,
		Err:      cmd.ErrOrStderr(),
	})
}

// askForValues asks for whatever the template needs and is not already set.
//
// The questions go to stderr, so that a piped `--dry-run` still receives only
// the rendered template.
func askForValues(cmd *cobra.Command, o tplutil.RenderOptions) (tplutil.Values, error) {
	vars, err := tplutil.Vars(o.Template)
	if err != nil {
		return nil, err
	}

	base, err := tplutil.BaseContext(o)
	if err != nil {
		return nil, err
	}

	prompt := cmd.ErrOrStderr()
	if len(vars) == 0 {
		fmt.Fprintf(prompt, "%s takes no values.\n", o.Template.Name)
		return o.Values, nil
	}
	fmt.Fprintf(prompt, "Values for %s:\n", o.Template.Name)

	delims, err := tplutil.Delimiters(o.Template)
	if err != nil {
		return nil, err
	}

	values, err := tplutil.Prompter{In: cmd.InOrStdin(), Out: prompt, Delims: delims}.Ask(vars, o.Values, base)
	if err != nil {
		return nil, err
	}
	fmt.Fprintln(prompt)
	return values, nil
}
