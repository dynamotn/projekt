package template

import (
	"github.com/spf13/cobra"
)

var (
	Cmd = &cobra.Command{
		Use:   "template",
		Short: "Create a template file from various sources",
	}
)
