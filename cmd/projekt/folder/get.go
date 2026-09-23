package folder

import (
	"io"

	"github.com/spf13/cobra"

	"gitlab.com/dynamo.foss/projekt/pkg/folderutil"
)

func NewFolderGetCmd(out io.Writer) *cobra.Command {
	o := folderutil.GetOptions{}

	cmd := &cobra.Command{
		Use:   "get [short name]",
		Short: "Get project folder by short name",
		Long: `Print the folder a short name resolves to, which is what ` + "`pj`" + ` calls.

The name "-" is the project you were in before this one, so ` + "`pj -`" + ` takes you
back, and typing it again brings you here, the way ` + "`cd -`" + ` does.

Every lookup is remembered, one entry per project; --no-record is for a
lookup that is not a jump.`,
		Args:    cobra.ExactArgs(1),
		Aliases: []string{"g"},
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) != 0 {
				return nil, cobra.ShellCompDirectiveNoFileComp
			}
			return compListShortNames(toComplete)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return folderutil.FindFolderByShortName(out, args[0], o)
		},
	}

	cmd.Flags().BoolVar(&o.NoRecord, "no-record", false, "Don't remember this lookup as a jump")

	return cmd
}
