package folder

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"gitlab.com/dynamo-tools/projekt/pkg/cli"
	"gitlab.com/dynamo-tools/projekt/pkg/folderutil"
)

type folderSyncOptions struct {
	dryRun bool
	forks  int
	pull   bool
	tags   []string
}

func NewFolderSyncCmd(out io.Writer) *cobra.Command {
	opts := &folderSyncOptions{}

	cmd := &cobra.Command{
		Use:     "sync",
		Aliases: []string{"s"},
		Short:   "Synchronize Git repositories defined in configuration",
		Long: `Synchronize Git repositories defined in the configuration file.
This command will:
- Clone missing repositories
- Add the configured remotes and working trees to existing ones
- With --pull, fetch the existing ones and fast-forward each clean branch

Missing repositories are cloned concurrently. Use --forks to change how many
run at once, or --forks 1 to clone them one after another.

--pull never merges or rebases. A branch with uncommitted changes, a detached
HEAD, no upstream, or commits of its own is fetched and reported as skipped,
and left where it was. Configured working trees are pulled the same way.

  projekt folder sync                 # clone what is missing
  projekt folder sync --pull          # and bring the rest up to date
  projekt folder sync --pull -t work  # only the folders tagged work

Use --dry-run to see what would be done without making changes.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runFolderSync(opts, out)
		},
	}

	f := cmd.Flags()
	f.BoolVar(&opts.dryRun, "dry-run", false, "Show what would be done without making changes")
	f.IntVar(&opts.forks, "forks", folderutil.DefaultSyncForks, "Number of repositories to clone concurrently")
	f.BoolVar(&opts.pull, "pull", false, "Fetch existing repositories and fast-forward clean branches")
	registerTagsFlag(cmd, &opts.tags, "sync")

	cli.SetColorAndStyles(cmd)
	return cmd
}

func runFolderSync(opts *folderSyncOptions, out io.Writer) error {
	// An explicit --forks 0 would otherwise fall back to the default, which is
	// not what someone who typed a number expects.
	if opts.forks < 1 {
		return fmt.Errorf("--forks must be at least 1, got %d", opts.forks)
	}

	if opts.dryRun {
		cli.Info("Running in dry-run mode...")
	}

	return folderutil.SyncGitRepos(folderutil.SyncOptions{
		DryRun: opts.dryRun,
		Forks:  opts.forks,
		Pull:   opts.pull,
		Tags:   opts.tags,
	})
}
