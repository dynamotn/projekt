package root

import (
	"io"
	"os"

	"github.com/spf13/cobra"

	"gitlab.com/dynamo.foss/projekt/pkg/cli"
	"gitlab.com/dynamo.foss/projekt/pkg/templates"
)

func NewInitCmd(out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:       "init",
		Short:     "Initialize to install other needed commands",
		Args:      cobra.ExactArgs(1),
		ValidArgs: []string{"bash", "fish"},
		RunE: func(cmd *cobra.Command, args []string) error {
			return templates.GenCommands(args[0], os.Stdout)
		},
	}

	cli.SetColorAndStyles(cmd)
	return cmd
}
