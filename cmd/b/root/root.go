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
		NewBoilerplateTrustCmd(out),
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
		if !needsConfig(cmd) {
			return nil
		}
		// Decoding is lazy, so do it now, or its error would get past this.
		lazypath.GetConfig()
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

// needsConfig says whether cmd works on the configuration: version and the
// commands cobra adds never do, so a broken config must not stop them.
func needsConfig(cmd *cobra.Command) bool {
	for ; cmd.HasParent(); cmd = cmd.Parent() {
		if !cmd.Parent().HasParent() {
			switch cmd.Name() {
			case "version", "completion", "help":
				return false
			}
		}
	}
	return true
}

func Execute() {
	cobra.OnInitialize(func() {
		// CfgFile is only ever set by --config, which must name a real file.
		lazypath.CfgFileFromFlag = lazypath.CfgFile != ""
		lazypath.InitConfig()
	})

	if err := NewRootCmd(os.Stdout).Execute(); err != nil {
		cli.Debug("%v", err)
		os.Exit(1)
	}
}
