package root

import (
	"io"

	"github.com/spf13/cobra"

	"gitlab.com/dynamo.foss/projekt/pkg/cli"
)

func NewTemplateCmd(out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "template",
		Aliases: []string{"t", "tpl"},
		Short:   "Create a template file from various sources",
	}

	cli.SetColorAndStyles(cmd)
	return cmd
}

func NewFastTemplateCmd(out io.Writer) *cobra.Command {
	cmd := NewTemplateCmd(out)

	cmd.Use = "t"
	cmd.Aliases = []string{}
	cmd.SilenceUsage = true

	return cmd
}
