package pkg

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	version = "0.1"

	subCmd = "version"
	versionDesc = "Print the version number of Projekt"
	versionLongDesc = `All software has versions. This is Projekt's`
	versionStr = "Projekt CLI v" + version

	VersionCmd = &cobra.Command{
		Use:   subCmd,
		Short: versionDesc,
		Long:  versionLongDesc,
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println(versionStr)
		},
	}
)
