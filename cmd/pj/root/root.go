package root

import (
	"io"
	"os"

	"github.com/spf13/cobra"

	projekt "gitlab.com/dynamo.foss/projekt/cmd/projekt/root"
	"gitlab.com/dynamo.foss/projekt/pkg/cli"
	"gitlab.com/dynamo.foss/projekt/pkg/lazypath"
)

func NewRootCmd(out io.Writer) *cobra.Command {
	rootCmd := projekt.NewFastPjCmd(out)

	f := rootCmd.PersistentFlags()
	cli.GetEnv().AddFlags(f)

	rootCmd.AddCommand(
		cli.NewVersionCmd(out),
	)

	return rootCmd
}

func Execute() {
	cobra.OnInitialize(lazypath.InitConfig)

	if err := NewRootCmd(os.Stdout).Execute(); err != nil {
		cli.Debug(err)
		os.Exit(1)
	}
}
