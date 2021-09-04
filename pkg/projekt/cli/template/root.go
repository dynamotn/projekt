package template

import (
	"github.com/spf13/cobra"
)

var (
	description = "Create a template file from various sources"
	Cmd = &cobra.Command{
		Use:     "template",
		Aliases: []string{"t", "tpl"},
		Short:   description,
	}

	FastCmd = &cobra.Command{
		Use:   "t",
		Short: description,
	}
)
