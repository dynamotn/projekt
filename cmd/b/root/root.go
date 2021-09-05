package root

import (
	"io"
	"os"

	"github.com/spf13/cobra"

	projekt "gitlab.com/dynamo.foss/projekt/cmd/projekt/root"
	"gitlab.com/dynamo.foss/projekt/pkg/cli"
)

func NewRootCmd(out io.Writer) *cobra.Command {
	rootCmd := projekt.NewFastBoilerplateCmd(out)

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
