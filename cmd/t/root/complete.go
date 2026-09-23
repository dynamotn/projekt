package root

import (
	"github.com/spf13/cobra"

	"gitlab.com/dynamo.foss/projekt/pkg/tplutil"
)

// completeTemplateNames completes the first argument with the templates of the
// store, so `t new <TAB>` offers what can actually be rendered.
func completeTemplateNames(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveDefault
	}

	templates, err := tplutil.List()
	if err != nil {
		return nil, cobra.ShellCompDirectiveError
	}

	names := make([]string, 0, len(templates))
	for _, tpl := range templates {
		names = append(names, tpl.Name+"\t"+string(tpl.Kind)+" template")
	}
	return names, cobra.ShellCompDirectiveNoFileComp
}
