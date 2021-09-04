package folder

import (
	"github.com/spf13/cobra"
)

var (
	Cmd = &cobra.Command{
		Use:   "folder",
		Short: "Manage your project folder",
	}
)

func init() {
	Cmd.AddCommand(addCmd)
}
