package folder

import (
	"io"

	"github.com/spf13/cobra"

	"gitlab.com/dynamo-tools/projekt/pkg/cli"
	"gitlab.com/dynamo-tools/projekt/pkg/folderutil"
)

func NewFolderCheckCmd(out io.Writer) *cobra.Command {
	var tags []string
	var output string

	cmd := &cobra.Command{
		Use: "check",
		// "status" used to mean this command. It now means `folder status`,
		// which reports the git state of every folder rather than verifying
		// the repositories the configuration declares.
		Aliases: []string{"c"},
		Short:   "Check status of Git repositories in configuration",
		Long: `Check the status of all Git repositories defined in the configuration.
This command will verify:
- Whether repositories exist
- Whether they are valid Git repositories
- Whether remote URLs match configuration

Use --tags to check only part of the configuration. With --output, the
findings are printed on stdout as one row each, instead of being logged.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			opts := folderutil.CheckOptions{Tags: tags, Out: out}
			if output != "" {
				format, err := cli.ParseOutputFormat(output)
				if err != nil {
					return err
				}
				opts.Output = format
			}
			return folderutil.CheckGitReposStatus(opts)
		},
	}

	registerTagsFlag(cmd, &tags, "check")
	cli.AddOutputFlag(cmd, &output, "")

	cli.SetColorAndStyles(cmd)
	return cmd
}
