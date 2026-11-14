package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// AddOutputFlag registers -o/--output on cmd, with its help text and shell
// completion. def is the default value; formats are the accepted values and
// default to OutputFormats.
func AddOutputFlag(cmd *cobra.Command, target *string, def string, formats ...string) {
	if len(formats) == 0 {
		formats = OutputFormats
	}
	cmd.Flags().StringVarP(target, "output", "o", def,
		fmt.Sprintf("Output format, one of: %s", strings.Join(formats, ", ")))

	if err := cmd.RegisterFlagCompletionFunc("output",
		func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
			return formats, cobra.ShellCompDirectiveNoFileComp
		}); err != nil {
		Warn("Cannot register completion for --output: %v", err)
	}
}
