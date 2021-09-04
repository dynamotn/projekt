package boilerplate

import (
	"github.com/spf13/cobra"
)

var (
	description = "Create boilerplate project folder of a language/framework/tool..."
	Cmd = &cobra.Command{
		Use:     "boilerplate",
		Aliases: []string{"b", "bpl"},
		Short:   description,
	}

	FastCmd = &cobra.Command{
		Use:   "b",
		Short: description,
	}
)
