package root

import (
	"io"
	"os"

	"github.com/spf13/cobra"

	"gitlab.com/dynamo.foss/projekt/pkg/cli"
	"gitlab.com/dynamo.foss/projekt/pkg/tplutil"
)

func NewProjektTemplateCmd(out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "template",
		Aliases: []string{"t", "tpl"},
		Short:   "Create a template file from various sources",
	}

	f := cmd.PersistentFlags()
	f.StringVar(&tplutil.TemplateDir, "template-dir", "", "Template folder (default is $XDG_DATA_HOME/projekt/templates)")

	cmd.AddCommand(
		NewTemplateListCmd(out),
		NewTemplateInitCmd(out),
		NewTemplateSyncCmd(out),
		NewTemplateNewCmd(out),
		NewTemplateApplyCmd(out),
		NewTemplateDiffCmd(out),
		NewTemplateAddCmd(out),
		NewTemplateShowCmd(out),
		NewTemplatePathCmd(out),
	)

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
		cli.Debug("%v", err)
		os.Exit(1)
	}
}
