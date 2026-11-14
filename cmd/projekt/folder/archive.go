package folder

import (
	"errors"
	"io"
	"time"

	"github.com/spf13/cobra"

	"gitlab.com/dynamo-tools/projekt/pkg/cli"
	"gitlab.com/dynamo-tools/projekt/pkg/folderutil"
)

const archiveLongHelp = `Move a finished project out of the way.

A project that is done still costs something: a name in completion, a row in
every listing, a wrong guess when you meant the other one. Deleting it is a
bigger decision than you want to make on a Friday.

  projekt folder archive old-service --dry-run
  projekt folder archive old-service
  projekt folder archive old-service --to ~/archive/2026

The folder is moved, not deleted, and the entry is dropped from the
configuration — in that order, so a move that fails leaves the project where
it was. Bringing one back is ` + "`mv`" + ` and ` + "`projekt folder add`" + `.

Archiving is refused when the repository has changes that are not committed,
commits that are not pushed, no upstream at all, or something on the stash.
That is the one mistake here that moving the folder back does not undo.
--force says you mean it.

A project with working trees, or one declared in an included file, is
refused whatever --force says: remove the trees first, or archive it in the
file that declares it.

The archived folder keeps .projekt/archived.yaml, which records where it came
from and its configuration entry. --list shows what the archive holds.

The archive folder is $XDG_DATA_HOME/projekt/archive unless --archive-dir,
$PROJEKT_ARCHIVE_DIR or --to says otherwise.`

func NewFolderArchiveCmd(out io.Writer) *cobra.Command {
	o := folderutil.ArchiveOptions{}
	var list bool
	output := string(folderutil.OutputTable)

	cmd := &cobra.Command{
		Use:     "archive [short name | --list]",
		Short:   "Move a finished project out of the way",
		Long:    archiveLongHelp,
		Args:    cobra.MaximumNArgs(1),
		Aliases: []string{"retire"},
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) != 0 {
				return nil, cobra.ShellCompDirectiveNoFileComp
			}
			return compListShortNames(toComplete)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if list {
				format, err := folderutil.ParseOutputFormat(output)
				if err != nil {
					return err
				}
				return listArchive(out, o, format)
			}
			if len(args) != 1 {
				return errors.New("name the project to archive, or use --list")
			}
			return folderutil.ArchiveFolder(out, args[0], o)
		},
	}

	f := cmd.Flags()
	f.StringVar(&o.To, "to", "", "Archive into this folder instead")
	f.BoolVarP(&o.Force, "force", "F", false, "Archive even with work that is not committed or not pushed")
	f.BoolVar(&o.DryRun, "dry-run", false, "Say what would happen without moving anything")

	f.BoolVar(&list, "list", false, "List what the archive holds instead of archiving")
	cli.AddOutputFlag(cmd, &output, output)

	return cmd
}

func listArchive(out io.Writer, o folderutil.ArchiveOptions, format cli.OutputFormat) error {
	items, err := folderutil.ListArchive(o)
	if err != nil {
		return err
	}
	view := cli.ListView{
		Columns: []cli.ListColumn{
			{Header: "NAME", Key: "name"},
			{Header: "ORIGINAL", Key: "original"},
			{Header: "ARCHIVED", Key: "archived"},
		},
		Ordered: true,
	}
	for _, item := range items {
		archived := ""
		if !item.At.IsZero() {
			archived = item.At.Local().Format(time.DateOnly)
		}
		view.AppendRow(item.Name, item.Original, archived)
	}
	return cli.EncodeList(out, view, cli.ListOutputOption{Output: format})
}
