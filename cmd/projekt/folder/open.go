package folder

import (
	"io"

	"github.com/spf13/cobra"

	"gitlab.com/dynamo.foss/projekt/pkg/folderutil"
)

const openLongHelp = `Open a project in your editor.

Nine times out of ten the thing after jumping to a project is opening it, and
that is two commands to do one thing.

  projekt folder open backend-api
  projekt folder open backend-api --with "code --new-window"
  projekt folder open backend-api --dry-run

The editor is $VISUAL, then $EDITOR, then vi, and the value may carry its own
arguments. It runs with the project as its working directory as well as its
argument, because half of them open the folder they are given and the other
half open whatever is in the current one.`

func NewFolderOpenCmd(out io.Writer) *cobra.Command {
	o := folderutil.OpenOptions{}

	cmd := &cobra.Command{
		Use:     "open [short name]",
		Short:   "Open a project in your editor",
		Long:    openLongHelp,
		Args:    cobra.ExactArgs(1),
		Aliases: []string{"o", "edit"},
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) != 0 {
				return nil, cobra.ShellCompDirectiveNoFileComp
			}
			return compListShortNames(toComplete)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			o.In, o.Out, o.Err = cmd.InOrStdin(), out, cmd.ErrOrStderr()
			return folderutil.OpenFolder(out, args[0], o)
		},
	}

	f := cmd.Flags()
	f.StringVar(&o.With, "with", "", "Open it with this instead of $VISUAL or $EDITOR")
	f.BoolVar(&o.DryRun, "dry-run", false, "Print the command instead of running it")

	return cmd
}
