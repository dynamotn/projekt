package folder

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

var (
	Path string
	addCmd = &cobra.Command{
		Use:   "add",
		Short: "Add your project folder to database",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println(strings.Join(args, " "))
			return nil
		},
	}
)
