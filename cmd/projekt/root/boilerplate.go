package root

import (
	"io"

	"github.com/spf13/cobra"

	"gitlab.com/dynamo.foss/projekt/pkg/cli"
)

func NewBoilerplateCmd(out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "boilerplate",
		Aliases: []string{"b", "bpl"},
		Short:   "Create boilerplate project folder of a language/framework/tool...",
	}

	cli.SetColorAndStyles(cmd)
	return cmd
}

func NewFastBoilerplateCmd(out io.Writer) *cobra.Command {
	cmd := NewBoilerplateCmd(out)

	cmd.Use = "b"
	cmd.Aliases = []string{}
	cmd.SilenceUsage = true

	return cmd
}
