package root

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"gitlab.com/dynamo-tools/projekt/pkg/tplutil"
)

const trustLongHelp = `Allow a template to run commands, as it is now.

A template runs commands in two ways: its ` + "`after`" + ` hooks, and the
` + "`output`" + ` function. A template you wrote yourself runs them freely. One that
sits in a git repository with a remote — a store cloned with ` + "`t init`" + ` — is
content that changes under you on every ` + "`t sync`" + `, so its commands run only
once you have trusted it, and only as long as it stays exactly as it was when
you did. Any change to the template, or to the shared pieces of the store,
takes the trust back.

Asked on a terminal, a command that is not trusted yet asks first. Anywhere
else it is refused, and this is how to allow it beforehand — in CI, say, before
` + "`t check`" + `.

Review first: ` + "`t show <template>`" + `, or the history of the store with
` + "`git -C \"$(t path)\" log -p`" + `.

Examples:

  t trust go-cli       # one template
  t trust              # every template of the store`

func NewTemplateTrustCmd(out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:               "trust [template...]",
		Short:             "Allow a template of a cloned store to run commands",
		Long:              trustLongHelp,
		ValidArgsFunction: completeTemplateNames,
		RunE: func(cmd *cobra.Command, args []string) error {
			templates, err := templatesNamed(args)
			if err != nil {
				return err
			}
			for _, tpl := range templates {
				if err := tplutil.Trust(tplutil.TemplateOrigin(tpl)); err != nil {
					return err
				}
				if err := printTrusted(out, tpl); err != nil {
					return err
				}
			}
			return nil
		},
	}
	return cmd
}

// templatesNamed returns the templates named, or the whole store.
func templatesNamed(names []string) ([]tplutil.Template, error) {
	if len(names) == 0 {
		return tplutil.List()
	}
	templates := make([]tplutil.Template, 0, len(names))
	for _, name := range names {
		tpl, err := tplutil.Get(name)
		if err != nil {
			return nil, err
		}
		templates = append(templates, tpl)
	}
	return templates, nil
}

// printTrusted names a trusted template and the hooks it will now run.
func printTrusted(out io.Writer, tpl tplutil.Template) error {
	if _, err := fmt.Fprintf(out, "Trusted %s\n", tpl.Name); err != nil {
		return err
	}
	manifest, err := tplutil.LoadManifest(tpl)
	if err != nil {
		return err
	}
	for _, command := range manifest.After {
		if _, err := fmt.Fprintf(out, "  after: %s\n", command); err != nil {
			return err
		}
	}
	return nil
}
