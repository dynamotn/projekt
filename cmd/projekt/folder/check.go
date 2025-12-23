package folder

import (
	"io"

	"github.com/spf13/cobra"

	"gitlab.com/dynamo.foss/projekt/pkg/cli"
	"gitlab.com/dynamo.foss/projekt/pkg/lazypath"
)

func NewFolderCheckCmd(out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "check",
		Aliases: []string{"c", "status"},
		Short:   "Check status of Git repositories in configuration",
		Long: `Check the status of all Git repositories defined in the configuration.
This command will verify:
- Whether repositories exist
- Whether they are valid Git repositories
- Whether remote URLs match configuration`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return lazypath.CheckGitReposStatus()
		},
	}

	cli.SetColorAndStyles(cmd)
	return cmd
}
