package root

import (
	"io"

	"github.com/spf13/cobra"

	"gitlab.com/dynamo.foss/projekt/pkg/tplutil"
)

const initLongHelp = `Clone a store of templates.

A template store is a folder of files, so it can simply be a repository — which
is how the same templates reach your other machine, and how a team shares one
set.

The repository is cloned into the template folder, with its history, so you can
edit a template and push it back. The folder must not already hold templates:
point --template-dir somewhere else, or move what is there aside.

The repository is a URL, or the "server:group/name" shorthand resolved against
the gitServers of your projekt configuration.

Examples:

  t init git@github.com:me/templates.git
  t init github:me/templates
  t init git@github.com:team/templates.git --ref main`

const syncLongHelp = `Bring the template store up to date with its remote.

Pulls fast-forward only: a store you have edited without committing is
something to sort out by hand, not something for a sync to guess at.

Examples:

  t sync
  t sync --dry-run`

func NewTemplateInitCmd(out io.Writer) *cobra.Command {
	var ref string

	cmd := &cobra.Command{
		Use:     "init [repository]",
		Short:   "Clone a store of templates",
		Long:    initLongHelp,
		Args:    cobra.ExactArgs(1),
		Aliases: []string{"clone"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return tplutil.InitStore(out, args[0], ref)
		},
	}
	cmd.Flags().StringVar(&ref, "ref", "", "Branch or tag to clone")

	return cmd
}

func NewTemplateSyncCmd(out io.Writer) *cobra.Command {
	var dryRun bool

	cmd := &cobra.Command{
		Use:     "sync",
		Short:   "Update the template store from its remote",
		Long:    syncLongHelp,
		Args:    cobra.NoArgs,
		Aliases: []string{"pull", "update"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return tplutil.SyncStore(out, dryRun)
		},
	}
	cmd.Flags().BoolVarP(&dryRun, "dry-run", "d", false, "Say what would be pulled, without pulling")

	return cmd
}
