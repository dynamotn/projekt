package folder

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"gitlab.com/dynamo.foss/projekt/pkg/cli"
	"gitlab.com/dynamo.foss/projekt/pkg/lazypath"
)

const pruneLongHelp = `Drop the configuration entries whose folder is gone.

A configuration grows: a project is archived, a clone is deleted, a machine
is reinstalled, and the entry stays behind. It costs a wrong suggestion in
completion and a jump that lands nowhere.

  projekt folder prune --dry-run   # what would go
  projekt folder prune             # and go it does

Only a path that is definitely not there is removed. A folder on a drive that
is not mounted right now reads as an error rather than as absent, and is left
alone: it is not gone, it is not here, and those are different.

Working trees are pruned too, since one removed with plain git leaves the same
kind of entry behind.`

func NewFolderPruneCmd(out io.Writer) *cobra.Command {
	dryRun := false

	cmd := &cobra.Command{
		Use:     "prune",
		Short:   "Drop the configuration entries whose folder is gone",
		Long:    pruneLongHelp,
		Args:    cobra.NoArgs,
		Aliases: []string{"clean"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if dryRun {
				return reportStale(out, lazypath.FindStale(), true)
			}

			pruned, err := lazypath.PruneStale()
			if err != nil {
				return err
			}
			return reportStale(out, pruned, false)
		},
	}

	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "List what would be dropped, without dropping it")

	return cmd
}

// reportStale prints what is, or would be, dropped.
func reportStale(out io.Writer, stale []lazypath.Stale, dryRun bool) error {
	if len(stale) == 0 {
		cli.Info("Nothing to prune")
		return nil
	}

	verb := "Dropped"
	if dryRun {
		verb = "Would drop"
	}
	for _, entry := range stale {
		if _, err := fmt.Fprintf(out, "%s %s %s: %s\n", verb, entry.Kind, entry.Name, entry.Path); err != nil {
			return err
		}
	}

	_, err := fmt.Fprintf(out, "%d entr%s\n", len(stale), plural(len(stale)))
	return err
}

func plural(count int) string {
	if count == 1 {
		return "y"
	}
	return "ies"
}
