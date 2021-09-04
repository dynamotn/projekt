package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"gitlab.com/dynamo.foss/project/pkg/project/cli/folder"
	"gitlab.com/dynamo.foss/project/pkg"
)

var (
	rootCmd = &cobra.Command{
		Use:   "project",
		Short: "A smart command to work with your project folder",
	}
	versionCmd = &cobra.Command{
		Use:   "version",
		Short: "Print the version number of Project",
		Long:  `All software has versions. This is Project's`,
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("Project CLI v" + pkg.Version)
		},
	}
)

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.AddCommand(folder.Cmd)
	rootCmd.AddCommand(versionCmd)
}
