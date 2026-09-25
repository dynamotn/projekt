package folder

import (
	"io"

	"github.com/spf13/cobra"

	"gitlab.com/dynamo-tools/projekt/pkg/folderutil"
)

const selectLongHelp = `Choose a project, and print where it is.

This is what ` + "`pj`" + ` with no argument calls. With a query it narrows the list
first, so it is also the way out of an ambiguous name.

  pj                       # choose from all of them
  projekt folder select api
  projekt folder select -t work

If fzf or sk is on PATH it is used; $PROJEKT_PICKER or --with names another.
Otherwise the projects are numbered and the answer is read from the terminal.

Only the path is printed, and the list is drawn on standard error, so that
` + "`cd \"$(projekt folder select)\"`" + ` does what it looks like it does.

One candidate needs no question, and neither does a query that is a whole
project name, however many other names contain it.`

func NewFolderSelectCmd(out io.Writer) *cobra.Command {
	o := folderutil.SelectOptions{}

	cmd := &cobra.Command{
		Use:     "select [query]",
		Short:   "Choose a project and print where it is",
		Long:    selectLongHelp,
		Args:    cobra.MaximumNArgs(1),
		Aliases: []string{"pick", "choose"},
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) != 0 {
				return nil, cobra.ShellCompDirectiveNoFileComp
			}
			return compListShortNames(toComplete)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 1 {
				o.Query = args[0]
			}
			o.In, o.Err = cmd.InOrStdin(), cmd.ErrOrStderr()

			return folderutil.SelectFolder(out, o)
		},
	}

	f := cmd.Flags()
	f.StringVar(&o.With, "with", "", "Interactive filter to use, instead of fzf or sk")
	f.BoolVar(&o.NoRecord, "no-record", false, "Don't remember this as a jump")
	registerTagsFlag(cmd, &o.Tags, "choose from")

	return cmd
}
