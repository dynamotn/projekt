package root

import (
	"io"
	"os"

	"github.com/spf13/cobra"

	"gitlab.com/dynamo-tools/projekt/pkg/bplutil"
	"gitlab.com/dynamo-tools/projekt/pkg/cli"
	"gitlab.com/dynamo-tools/projekt/pkg/lazypath"
	"gitlab.com/dynamo-tools/projekt/pkg/tplutil"
)

func NewProjektBoilerplateCmd(out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "boilerplate",
		Aliases: []string{"b", "bpl"},
		Short:   "Create boilerplate project folder of a language/framework/tool...",
	}

	f := cmd.PersistentFlags()
	f.StringVar(&bplutil.RecipeDir, "boilerplate-dir", "", "Boilerplate folder (default is $XDG_DATA_HOME/projekt/boilerplates)")
	// A recipe creates from a template of the `t` store, so `b` needs to know
	// where that store is too.
	f.StringVar(&tplutil.TemplateDir, "template-dir", "", "Template folder (default is $XDG_DATA_HOME/projekt/templates)")

	cmd.AddCommand(
		NewBoilerplateListCmd(out),
		NewBoilerplateNewCmd(out),
		NewBoilerplateApplyCmd(out),
		NewBoilerplateCheckCmd(out),
		NewBoilerplateShowCmd(out),
		NewBoilerplatePathCmd(out),
	)

	cli.SetColorAndStyles(cmd)
	return cmd
}

func NewRootCmd(out io.Writer) *cobra.Command {
	rootCmd := NewProjektBoilerplateCmd(out)

	rootCmd.Use = "b"
	rootCmd.Aliases = []string{}
	rootCmd.SilenceUsage = true
	// `b` writes to the configuration when it registers a project, so it
	// refuses to run on a config it could not read, like projekt itself.
	rootCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		return lazypath.LoadError()
	}

	f := rootCmd.PersistentFlags()
	f.StringVar(&lazypath.CfgFile, "config", "", "config file (default is $XDG_CONFIG_HOME/projekt/config.yaml)")
	cli.GetEnv().AddFlags(f)

	rootCmd.AddCommand(
		cli.NewVersionCmd(out),
	)

	return rootCmd
}

func Execute() {
	cobra.OnInitialize(lazypath.InitConfig)

	if err := NewRootCmd(os.Stdout).Execute(); err != nil {
		cli.Debug("%v", err)
		os.Exit(1)
	}
}
