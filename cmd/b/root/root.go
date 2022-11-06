package root

import (
	"io"
	"os"

	"github.com/spf13/cobra"

	"gitlab.com/dynamo.foss/projekt/pkg/cli"
)

func NewProjektBoilerplateCmd(out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "boilerplate",
		Aliases: []string{"b", "bpl"},
		Short:   "Create boilerplate project folder of a language/framework/tool...",
	}

	cli.SetColorAndStyles(cmd)
	return cmd
}

func NewRootCmd(out io.Writer) *cobra.Command {
	rootCmd := NewProjektBoilerplateCmd(out)

	rootCmd.Use = "b"
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
