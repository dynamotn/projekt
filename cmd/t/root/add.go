package root

import (
	"errors"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"gitlab.com/dynamo-tools/projekt/pkg/cli"
	"gitlab.com/dynamo-tools/projekt/pkg/tplutil"
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
    --replace 'Jane Doe=Values.author'

--suggest asks the AI command you name in ai.command (or PROJEKT_AI) which
literals of the project should become values. Each pair is checked, printed
with the matching --replace line, and the template is added only on a yes at
a terminal or with --yes. --show-prompt prints what would be sent. The files
of a nested repository are left out of what is sent.`

func NewTemplateAddCmd(out io.Writer) *cobra.Command {
	var replace []string
	var suggest, yes, showPrompt bool
	o := tplutil.AddOptions{}

	cmd := &cobra.Command{
		Use:     "add [file or folder]",
		Short:   "Save an existing file or folder as a template",
		Long:    addLongHelp,
		Args:    cobra.ExactArgs(1),
		Aliases: []string{"a", "import"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if showPrompt && !suggest {
				return errors.New("--show-prompt needs --suggest")
			}
			replacements, err := tplutil.ParseReplacements(replace)
			if err != nil {
				return err
			}
			if suggest {
				suggested, err := tplutil.SuggestReplacements(args[0], tplutil.SuggestOptions{
					ShowPrompt: showPrompt, Given: replacements, Out: out, Err: cmd.ErrOrStderr(),
				})
				if err != nil || showPrompt {
					return err
				}
				if len(suggested) == 0 && yes {
					return errors.New("no suggestion held; nothing added")
				}
				if len(suggested) == 0 {
					return nil
				}
				// What was typed wins over what was suggested for one literal.
				given := map[string]bool{}
				for _, r := range replacements {
					given[r.Literal] = true
				}
				for _, r := range suggested {
					if !given[r.Literal] {
						replace = append(replace, r.Literal+"="+r.Expr)
					}
				}
				if replacements, err = tplutil.ParseReplacements(replace); err != nil {
					return err
				}
				if !yes {
					// Without a person to ask, the suggestion is only shown.
					if ok, err := cli.Confirm(cmd.InOrStdin(), cmd.ErrOrStderr(), "Add the template with these replacements? [y/N] "); err != nil || !ok {
						return err
					}
				}
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

	f.BoolVar(&suggest, "suggest", false, "Ask the AI command which literals should become values, then add on a yes")
	f.BoolVarP(&yes, "yes", "y", false, "With --suggest, add without asking")
	f.BoolVar(&showPrompt, "show-prompt", false, "With --suggest, print the prompt and stop")

	return cmd
}
