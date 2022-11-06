package root

import (
	"io"
	"os"

	"github.com/spf13/cobra"

	"gitlab.com/dynamo.foss/projekt/pkg/cli"
)

func NewProjektTemplateCmd(out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "template",
		Aliases: []string{"t", "tpl"},
		Short:   "Create a template file from various sources",
	}

	cmd.AddCommand()

	cli.SetColorAndStyles(cmd)
	return cmd
}

func NewRootCmd(out io.Writer) *cobra.Command {
	rootCmd := NewProjektTemplateCmd(out)

	rootCmd.Use = "t"
	rootCmd.Aliases = []string{}
	rootCmd.SilenceUsage = true

	f := rootCmd.PersistentFlags()
	cli.GetEnv().AddFlags(f)

	rootCmd.AddCommand(
		cli.NewVersionCmd(out),
	)

	return rootCmd
}

func Execute() {
	if err := NewRootCmd(os.Stdout).Execute(); err != nil {
		cli.Debug(err)
		os.Exit(1)
	}
}
