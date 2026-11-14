package folder

import (
	"io"

	"github.com/spf13/cobra"

	"gitlab.com/dynamo-tools/projekt/pkg/folderutil"
)

const restoreLongHelp = `Bring an archived project back.

  projekt folder archive --list
  projekt folder restore old-service --dry-run
  projekt folder restore old-service
  projekt folder restore old-service --to ~/work/old-service

The folder is moved back to the path its record names, which is shown and
confirmed first (--yes skips the question); its parent folder must exist.
With --to the parents are created and nothing is asked. The configuration
entry is put back with its name, prefix, tags, description, priority and
workspace settings. Hooks and a git section are not restored, since the
record is a file anyone may have edited: they are printed, so that you can
add them back with ` + "`projekt config edit`" + ` after reading them. Then the record
is removed, so the project does not look unfinished to a later archive. A
folder with no entry, a child of a workspace, is registered the way
` + "`projekt folder add <path>`" + ` would.

Nothing changes when the path already exists, when the configuration already
has an entry for it, or when the record is missing, links elsewhere or names
no absolute path outside the archive. A project archived before records were
kept needs --to.

The archive folder is the one --archive-dir, $PROJEKT_ARCHIVE_DIR or --from
names, else $XDG_DATA_HOME/projekt/archive.`

func NewFolderRestoreCmd(out io.Writer) *cobra.Command {
	o := folderutil.RestoreOptions{}

	cmd := &cobra.Command{
		Use:   "restore [name]",
		Short: "Bring an archived project back",
		Long:  restoreLongHelp,
		Args:  cobra.ExactArgs(1),
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) != 0 {
				return nil, cobra.ShellCompDirectiveNoFileComp
			}
			items, err := folderutil.ListArchive(folderutil.ArchiveOptions{To: o.From})
			if err != nil {
				return nil, cobra.ShellCompDirectiveNoFileComp
			}
			var names []string
			for _, item := range items {
				names = append(names, item.Name)
			}
			return names, cobra.ShellCompDirectiveNoFileComp
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			o.In, o.Err = cmd.InOrStdin(), cmd.ErrOrStderr()
			return folderutil.RestoreFolder(out, args[0], o)
		},
	}

	f := cmd.Flags()
	f.StringVar(&o.From, "from", "", "Restore from this archive folder instead")
	f.StringVar(&o.To, "to", "", "Restore to this path instead of the one the record names")
	f.BoolVarP(&o.Yes, "yes", "y", false, "Do not ask to confirm a path taken from the record")
	f.BoolVar(&o.DryRun, "dry-run", false, "Say what would happen without changing anything")

	return cmd
}
