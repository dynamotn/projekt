package root

import (
	"io"
	"os"

	"github.com/spf13/cobra"

	config "gitlab.com/dynamo.foss/projekt/cmd/projekt/config"
	folder "gitlab.com/dynamo.foss/projekt/cmd/projekt/folder"
	worktree "gitlab.com/dynamo.foss/projekt/cmd/projekt/worktree"
	"gitlab.com/dynamo.foss/projekt/pkg/cli"
	"gitlab.com/dynamo.foss/projekt/pkg/lazypath"
)

func NewRootCmd(out io.Writer) *cobra.Command {
	rootCmd := &cobra.Command{
		Use:          "projekt",
		Short:        "A smart command to work with your project folder",
		SilenceUsage: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			// Stop before running anything on a config we could not read,
			// instead of silently behaving as if it were empty.
			return lazypath.LoadError()
		},
	}

	f := rootCmd.PersistentFlags()
	f.StringVar(&lazypath.CfgFile, "config", "", "config file (default is $XDG_CONFIG_HOME/projekt/config.yaml)")
	cli.GetEnv().AddFlags(f)

	rootCmd.AddCommand(
		NewInitCmd(out),
		folder.NewFolderCmd(out),
		worktree.NewWorktreeCmd(out),
		config.NewConfigCmd(out),
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
