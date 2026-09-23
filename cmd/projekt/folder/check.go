package folder

import (
	"io"

	"github.com/spf13/cobra"

	"gitlab.com/dynamo.foss/projekt/pkg/cli"
	"gitlab.com/dynamo.foss/projekt/pkg/folderutil"
)

func NewFolderCheckCmd(out io.Writer) *cobra.Command {
	var tags []string

	cmd := &cobra.Command{
		Use:     "check",
		Aliases: []string{"c", "status"},
		Short:   "Check status of Git repositories in configuration",
		Long: `Check the status of all Git repositories defined in the configuration.
This command will verify:
- Whether repositories exist
- Whether they are valid Git repositories
- Whether remote URLs match configuration

Use --tags to check only part of the configuration.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return folderutil.CheckGitReposStatus(folderutil.CheckOptions{Tags: tags})
		},
	}

	registerTagsFlag(cmd, &tags, "check")

	cli.SetColorAndStyles(cmd)
	return cmd
}
