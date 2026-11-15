package folder

import (
	"io"

	"github.com/spf13/cobra"

	"gitlab.com/dynamo-tools/projekt/pkg/cli"
	"gitlab.com/dynamo-tools/projekt/pkg/folderutil"
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
usable as a check. Working trees are projects too, so they are included.

--timeout stops the command in a folder that runs longer, together with
everything it started, and reports it as timed out. --fail-fast starts no new
folder after the first failure; the ones already running finish, and the rest
are reported as not started. With --timeout the command runs in a process
group of its own, so one that asks the terminal for input is stopped and
reported as timed out: leave --timeout off for an interactive command.

With -o json, stdout holds one array with an object per folder (name, path,
exit, output, seconds), in the same order. exit is -1 when the command could
not start, timed out, was interrupted or was not started because of
--fail-fast, and when the folder is missing.`

func NewFolderExecCmd(out io.Writer) *cobra.Command {
	o := folderutil.ExecOptions{}
	var output string

	cmd := &cobra.Command{
		Use:     "exec -- [command]",
		Short:   "Run a command in every project folder",
		Long:    execLongHelp,
		Args:    cobra.MinimumNArgs(1),
		Aliases: []string{"e", "each", "foreach"},
		RunE: func(cmd *cobra.Command, args []string) error {
			o.Command = args
			o.Output = cli.OutputFormat(output)
			return folderutil.ExecInFolders(out, o)
		},
	}

	f := cmd.Flags()
	// Flags after the command are the command's: `exec rg -q foo` is not --quiet.
	f.SetInterspersed(false)
	f.IntVar(&o.Forks, "forks", folderutil.DefaultSyncForks, "How many folders to work on at once")
	f.BoolVarP(&o.Quiet, "quiet", "q", false, "Report only the folders where the command failed")
	f.BoolVar(&o.NoOutput, "no-output", false, "Report the verdict per folder, not the command's output")
	f.BoolVar(&o.DryRun, "dry-run", false, "List what would be run, without running any of it")
	f.DurationVar(&o.Timeout, "timeout", 0, "Stop the command in a folder after this long, 0 for no limit")
	f.BoolVar(&o.FailFast, "fail-fast", false, "Start no new folder after the first failure")
	cli.AddOutputFlag(cmd, &output, "", string(cli.OutputJSON))
	registerTagsFlag(cmd, &o.Tags, "run in")

	return cmd
}
