package folder

import (
	"github.com/spf13/cobra"
)

var (
	Cmd = &cobra.Command{
		Use:     "folder",
		Aliases: []string{"f", "fd", "fol"},
		Short:   "Manage your project folder",
	}
)

func init() {
	Cmd.AddCommand(addCmd)
}
