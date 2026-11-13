package root

import (
	"io"
	"os"
	"slices"

	"github.com/spf13/cobra"

	config "gitlab.com/dynamo-tools/projekt/cmd/projekt/config"
	folder "gitlab.com/dynamo-tools/projekt/cmd/projekt/folder"
	worktree "gitlab.com/dynamo-tools/projekt/cmd/projekt/worktree"
	"gitlab.com/dynamo-tools/projekt/pkg/cli"
	"gitlab.com/dynamo-tools/projekt/pkg/lazypath"
)

func NewRootCmd(out io.Writer) *cobra.Command {
	rootCmd := &cobra.Command{
		Use:          "projekt",
		Short:        "A smart command to work with your project folder",
		SilenceUsage: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if !needsConfig(cmd, "template") {
				return nil
			}
			// Stop before running anything on a config we could not read,
			// instead of silently behaving as if it were empty. Decoding is
			// lazy, so do it now, or its error would get past this check.
			lazypath.GetConfig()
			return lazypath.LoadError()
		},
	}

	f := rootCmd.PersistentFlags()
	f.StringVar(&lazypath.CfgFile, "config", "", "config file (default is $XDG_CONFIG_HOME/projekt/config.yaml)")
	cli.GetEnv().AddFlags(f)

	rootCmd.AddCommand(
		NewInitCmd(out),
		NewDoctorCmd(out),
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

// needsConfig says whether cmd works on the configuration. The commands cobra
// adds, version, and those named in exempt never do, so a broken config must
// not stop them.
func needsConfig(cmd *cobra.Command, exempt ...string) bool {
	for ; cmd.HasParent(); cmd = cmd.Parent() {
		if cmd.Parent().HasParent() {
			continue
		}
		switch cmd.Name() {
		case "version", "completion", "help":
			return false
		}
		return !slices.Contains(exempt, cmd.Name())
	}
	return true
}

func Execute() {
	cobra.OnInitialize(lazypath.InitConfig)

	if err := NewRootCmd(os.Stdout).Execute(); err != nil {
		cli.Debug("%v", err)
		os.Exit(1)
	}
}
