package root

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"gitlab.com/dynamo.foss/projekt/pkg/tplutil"
)

const addLongHelp = `Save an existing file or folder as a template.

The content is copied as it is, so any template action already in it is kept.
That is half the job: the project also says its own name, its own module path
and its own author in every second file, and a template says ` + "`{{ .Name }}`" + `.

--replace does that half. Each one is literal=Expr, where Expr is a template
path: .Name and the rest come for free, anything under .Values becomes a
question, and a ` + "`.vars.yaml`" + ` is written with what the project already said
as the default. The literals are replaced in the file names as well as in the
contents, longest first, and a file that is not text is copied untouched.

Examples:

  t add ./LICENSE
  t add ./myapp --name go-cli
  t add ./myapp --name go-cli \
    --replace myapp=Name \
    --replace example.com/myapp=Values.module \
    --replace 'Jane Doe=Values.author'`

func NewTemplateAddCmd(out io.Writer) *cobra.Command {
	var replace []string
	o := tplutil.AddOptions{}

	cmd := &cobra.Command{
		Use:     "add [file or folder]",
		Short:   "Save an existing file or folder as a template",
		Long:    addLongHelp,
		Args:    cobra.ExactArgs(1),
		Aliases: []string{"a", "import"},
		RunE: func(cmd *cobra.Command, args []string) error {
			replacements, err := tplutil.ParseReplacements(replace)
			if err != nil {
				return err
			}
			o.Source, o.Replace = args[0], replacements

			added, err := tplutil.Add(o)
			if err != nil {
				return err
			}
			if _, err := fmt.Fprintln(out, added.Path); err != nil {
				return err
			}
			if added.Substitutions > 0 {
				if _, err := fmt.Fprintf(cmd.ErrOrStderr(), "%d substitution(s) in %d file(s)\n",
					added.Substitutions, added.Files); err != nil {
					return err
				}
			}
			if added.Manifest != "" {
				_, err = fmt.Fprintf(cmd.ErrOrStderr(), "Questions written to %s\n", added.Manifest)
			}
			return err
		},
	}

	f := cmd.Flags()
	f.StringVarP(&o.Name, "name", "n", "", "Name of the template, defaults to the source name")
	f.StringArrayVarP(&replace, "replace", "r", nil, "Turn a literal into a template expression, like -r myapp=Name (repeatable)")
	f.BoolVarP(&o.Force, "force", "F", false, "Overwrite a template that already exists")

	return cmd
}
