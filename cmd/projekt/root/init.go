package root

import (
	"io"

	"github.com/spf13/cobra"

	"gitlab.com/dynamo.foss/projekt/pkg/cli"
	"gitlab.com/dynamo.foss/projekt/pkg/templates"
)

func NewInitCmd(out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init [shell]",
		Short: "Initialize to install other needed commands",
		// OnlyValidArgs is required for ValidArgs to actually reject a shell we
		// don't ship a template for.
		Args:      cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
		ValidArgs: []string{"bash", "fish"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return templates.GenCommands(args[0], out)
		},
	}

	cli.SetColorAndStyles(cmd)
	return cmd
}
