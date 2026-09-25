package config

import (
	"io"

	"github.com/spf13/cobra"

	"gitlab.com/dynamo-tools/projekt/pkg/cli"
)

func NewConfigCmd(out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "config",
		Aliases: []string{"cfg"},
		Short:   "Inspect and edit your configuration file",
	}

	cmd.AddCommand(
		NewConfigCheckCmd(out),
		NewConfigEditCmd(out),
	)

	cli.SetColorAndStyles(cmd)
	return cmd
}
