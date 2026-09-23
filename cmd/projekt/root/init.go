package root

import (
	"io"
	"strings"

	"github.com/spf13/cobra"

	"gitlab.com/dynamo.foss/projekt/pkg/cli"
	"gitlab.com/dynamo.foss/projekt/pkg/templates"
)

func NewInitCmd(out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init [shell]",
		Short: "Initialize to install other needed commands",
		Long: "Print the shell integration to source, which installs `pj`\n" +
			"and its completion.\n\n" +
			"Available shells: " + strings.Join(templates.Shells(), ", "),
		// OnlyValidArgs is required for ValidArgs to actually reject a shell we
		// don't ship a template for.
		Args:      cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
		ValidArgs: templates.Shells(),
		RunE: func(cmd *cobra.Command, args []string) error {
			return templates.GenCommands(args[0], out)
		},
	}

	cli.SetColorAndStyles(cmd)
	return cmd
}
