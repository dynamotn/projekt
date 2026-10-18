package folder

import (
	"io"
	"os"

	"github.com/spf13/cobra"

	"gitlab.com/dynamo-tools/projekt/pkg/folderutil"
)

const noteLongHelp = `Keep a note with a project: what is half-done, what to pick up next.

The note is a Markdown file at ` + folderutil.NoteFile + ` inside the project,
so it moves, archives and is backed up with the project rather than with the
configuration. With no project named, the one the current folder is in is
meant.

  projekt folder note                         # read the note of this project
  projekt folder note billing                 # or of that one
  projekt folder note -a "migration is half done, see PR 42"
  projekt folder note billing --edit          # open it in $VISUAL or $EDITOR
  projekt folder status --notes               # the first line of every note

--add writes a dated bullet at the end, so the note doubles as a log of where
you left things. Reading a project with no note exits non-zero.`

func NewFolderNoteCmd(out io.Writer) *cobra.Command {
	o := folderutil.NoteOptions{In: os.Stdin, Out: os.Stdout, Err: os.Stderr}

	cmd := &cobra.Command{
		Use:     "note [short name]",
		Short:   "Read, extend or edit the note kept with a project",
		Long:    noteLongHelp,
		Args:    cobra.MaximumNArgs(1),
		Aliases: []string{"notes"},
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) == 0 {
				return compListShortNames(toComplete)
			}
			return nil, cobra.ShellCompDirectiveNoFileComp
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			shortName := ""
			if len(args) == 1 {
				shortName = args[0]
			}
			return folderutil.NoteFolder(out, shortName, o)
		},
	}

	f := cmd.Flags()
	f.StringVarP(&o.Add, "add", "a", "", "Append this as a dated entry")
	f.BoolVarP(&o.Edit, "edit", "e", false, "Open the note in the editor")

	return cmd
}
