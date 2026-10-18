package folder

import (
	"errors"
	"io"

	"github.com/spf13/cobra"

	"gitlab.com/dynamo-tools/projekt/pkg/hookutil"
)

const hooksLongHelp = `Print the shell lines a jump should run, or list a project's hooks.

A project can declare lines for the shell to evaluate when you jump into it
and when you jump out of it — ` + "`nvm use`" + `, a KUBECONFIG, a virtualenv:

  folders:
    - path: ~/work/api
      tags: [k8s]
      hooks:
        enter: ["export KUBECONFIG=~/.kube/api"]
        leave: ["unset KUBECONFIG"]
  hooks:
    - tags: [k8s]
      enter: ["kubectl config use-context dev"]

The ` + "`pj`" + ` function calls this with --from and --to after every jump and
evaluates what it prints: the leave hooks of the project being left, then the
enter hooks of the one being entered, folder entry first and tag rules after.
Staying inside one project runs nothing. Set ` + hookutil.NoHooksEnv + ` to turn them
all off.

  projekt folder hooks api                 # what would run, and why
  projekt folder hooks --from "$PWD" --to ~/work/api

A line from an included file that sits in a repository is handed over only
once that version of the file is trusted: on a terminal you are asked, and
elsewhere the line is reported and left out, so that the jump still happens.
'projekt config trust <file>' allows it beforehand.`

func NewFolderHooksCmd(out io.Writer) *cobra.Command {
	o := hookutil.TransitionOptions{}

	cmd := &cobra.Command{
		Use:   "hooks [short name]",
		Short: "Print the shell lines a jump runs, or list a project's hooks",
		Long:  hooksLongHelp,
		Args:  cobra.MaximumNArgs(1),
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) == 0 {
				return compListShortNames(toComplete)
			}
			return nil, cobra.ShellCompDirectiveNoFileComp
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 1 {
				if o.To != "" || o.From != "" {
					return errors.New("name a project, or give --from and --to, not both")
				}
				return hookutil.Show(out, args[0])
			}
			if o.To == "" {
				return errors.New("--to is required when no project is named")
			}
			return hookutil.Transition(out, o)
		},
	}

	f := cmd.Flags()
	f.StringVar(&o.From, "from", "", "The folder being left")
	f.StringVar(&o.To, "to", "", "The folder being entered")

	return cmd
}
