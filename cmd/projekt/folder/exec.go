package folder

import (
	"io"

	"github.com/spf13/cobra"

	"gitlab.com/dynamo.foss/projekt/pkg/folderutil"
)

const execLongHelp = `Run one command in every project folder.

The question this answers is the one you cannot answer by jumping: which of my
projects is dirty, which is behind, which still pins the old version.

  projekt folder exec -- git status --short
  projekt folder exec -t work -- git fetch --quiet
  projekt folder exec --quiet -- git diff --quiet   # only the dirty ones
  projekt folder exec -- rg -l "old-api"

Everything after -- is the command. Folders are worked on several at a time,
but reported in configuration order, so the output can be compared between
runs. A folder that is not on disk is reported as missing rather than as a
failure of the command.

The exit code is non-zero when the command failed anywhere, which makes it
usable as a check. Working trees are projects too, so they are included.`

func NewFolderExecCmd(out io.Writer) *cobra.Command {
	o := folderutil.ExecOptions{}

	cmd := &cobra.Command{
		Use:     "exec -- [command]",
		Short:   "Run a command in every project folder",
		Long:    execLongHelp,
		Args:    cobra.MinimumNArgs(1),
		Aliases: []string{"e", "each", "foreach"},
		RunE: func(cmd *cobra.Command, args []string) error {
			o.Command = args
			return folderutil.ExecInFolders(out, o)
		},
	}

	f := cmd.Flags()
	f.IntVar(&o.Forks, "forks", folderutil.DefaultSyncForks, "How many folders to work on at once")
	f.BoolVarP(&o.Quiet, "quiet", "q", false, "Report only the folders where the command failed")
	f.BoolVar(&o.NoOutput, "no-output", false, "Report the verdict per folder, not the command's output")
	f.BoolVar(&o.DryRun, "dry-run", false, "List what would be run, without running any of it")
	registerTagsFlag(cmd, &o.Tags, "run in")

	return cmd
}
