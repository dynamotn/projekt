package cli

import (
	"fmt"
	"os"

	"gitlab.com/dynamo.foss/projekt/pkg"
	"gitlab.com/dynamo.foss/projekt/pkg/projekt/cli/template"
)

var (
	RootCmd = template.FastCmd
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
	RootCmd.AddCommand(pkg.VersionCmd)
	pkg.SetColorAndStyles(RootCmd)
}
