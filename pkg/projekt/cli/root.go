package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"gitlab.com/dynamo.foss/projekt/pkg"
	"gitlab.com/dynamo.foss/projekt/pkg/projekt"
	"gitlab.com/dynamo.foss/projekt/pkg/projekt/cli/folder"
	"gitlab.com/dynamo.foss/projekt/pkg/projekt/cli/template"
	"gitlab.com/dynamo.foss/projekt/pkg/projekt/cli/boilerplate"
)

var (
	RootCmd = &cobra.Command{
		Use:   "projekt",
		Short: "A smart command to work with your project folder",
	}
)

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the RootCmd.
func Execute() {
	if err := RootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(projekt.InitConfig)

	RootCmd.PersistentFlags().StringVar(&projekt.CfgFile, "config", "", "config file (default is $HOME/.projekt/config.yaml)")

	RootCmd.AddCommand(folder.Cmd)
	RootCmd.AddCommand(template.Cmd)
	RootCmd.AddCommand(boilerplate.Cmd)
	RootCmd.AddCommand(pkg.VersionCmd)

	pkg.SetColorAndStyles(RootCmd)
}
