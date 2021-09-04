package boilerplate

import (
	"github.com/spf13/cobra"
)

var (
	Cmd = &cobra.Command{
		Use:   "boilerplate",
		Short: "Create boilerplate project folder of a language/framework/tool...",
	}
)
