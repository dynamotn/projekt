package folder

import (
	"io"

	"github.com/spf13/cobra"

	"gitlab.com/dynamo.foss/projekt/pkg/folderutil"
)

const currentLongHelp = `Say which project this folder is in.

Everything else here answers "where is it". This answers the other half: am I
in one, and which.

  projekt folder current            # the short name
  projekt folder current --path     # where it is
  projekt folder current /some/path # ask about somewhere else
  projekt folder current -q || echo "not in a project"

The deepest match wins, so standing in a working tree names the working tree
rather than the project it hangs off: both contain you, and the closer one is
the answer.

It exits non-zero when the folder is not in a project, which is what makes it
usable in a prompt or a script.`

func NewFolderCurrentCmd(out io.Writer) *cobra.Command {
	o := folderutil.CurrentOptions{}

	cmd := &cobra.Command{
		Use:     "current [path]",
		Short:   "Say which project this folder is in",
		Long:    currentLongHelp,
		Args:    cobra.MaximumNArgs(1),
		Aliases: []string{"where", "here"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 1 {
				o.From = args[0]
			}
			return folderutil.CurrentFolder(out, o)
		},
	}

	f := cmd.Flags()
	f.BoolVarP(&o.PrintPath, "path", "p", false, "Print where the project is, not what it is called")
	f.BoolVarP(&o.Quiet, "quiet", "q", false, "Print nothing; answer with the exit code")

	return cmd
}
