package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"gitlab.com/dynamo.foss/projekt/pkg"
)

var (
	RootCmd = &cobra.Command{
		Use:   "pj",
		Short: "Go to project",
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
	RootCmd.AddCommand(pkg.VersionCmd)
	pkg.SetColorAndStyles(RootCmd)
}
