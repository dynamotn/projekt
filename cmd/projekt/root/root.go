package root

import (
	"io"
	"os"

	"github.com/spf13/cobra"

	folder "gitlab.com/dynamo.foss/projekt/cmd/projekt/folder"
	"gitlab.com/dynamo.foss/projekt/pkg/cli"
	"gitlab.com/dynamo.foss/projekt/pkg/lazypath"
)

func NewRootCmd(out io.Writer) *cobra.Command {
	rootCmd := &cobra.Command{
		Use:          "projekt",
		Short:        "A smart command to work with your project folder",
		SilenceUsage: true,
	}

	f := rootCmd.PersistentFlags()
	f.StringVar(&lazypath.CfgFile, "config", "", "config file (default is $XDG_CONFIG_HOME/projekt/config.yaml)")
	cli.GetEnv().AddFlags(f)

	rootCmd.AddCommand(
		NewInitCmd(out),
		folder.NewFolderCmd(out),
		NewTemplateCmd(out),
		NewBoilerplateCmd(out),
		cli.NewVersionCmd(out),
	)
	cli.SetColorAndStyles(rootCmd)

	return rootCmd
}

func Execute() {
	cobra.OnInitialize(lazypath.InitConfig)

	if err := NewRootCmd(os.Stdout).Execute(); err != nil {
		cli.Debug("%v", err)
		os.Exit(1)
	}
}
