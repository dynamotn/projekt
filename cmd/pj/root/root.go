package root

import (
	"io"
	"os"

	"github.com/spf13/cobra"

	"gitlab.com/dynamo.foss/projekt/pkg/cli"
)

func NewRootCmd(out io.Writer) *cobra.Command {
	rootCmd := &cobra.Command{
		Use:          "pj",
		Short:        "Go to project folder",
		SilenceUsage: true,
	}

	f := rootCmd.PersistentFlags()
	cli.GetEnv().AddFlags(f)

	rootCmd.AddCommand(
		cli.NewVersionCmd(out),
	)
	cli.SetColorAndStyles(rootCmd)

	//TODO: Add completion of list folder

	return rootCmd
}

func Execute() {
	if err := NewRootCmd(os.Stdout).Execute(); err != nil {
		cli.Debug(err)
		os.Exit(1)
	}
}
