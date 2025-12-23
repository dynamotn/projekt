package folder

import (
	"io"

	"github.com/spf13/cobra"

	"gitlab.com/dynamo.foss/projekt/pkg/cli"
	"gitlab.com/dynamo.foss/projekt/pkg/lazypath"
)

type folderSyncOptions struct {
	dryRun bool
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
- Check existing repositories

Use --dry-run to see what would be done without making changes.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runFolderSync(opts, out)
		},
	}

	f := cmd.Flags()
	f.BoolVar(&opts.dryRun, "dry-run", false, "Show what would be done without making changes")

	cli.SetColorAndStyles(cmd)
	return cmd
}

func runFolderSync(opts *folderSyncOptions, out io.Writer) error {
	if opts.dryRun {
		cli.Info("Running in dry-run mode...")
	}

	return lazypath.SyncGitRepos(opts.dryRun)
}
