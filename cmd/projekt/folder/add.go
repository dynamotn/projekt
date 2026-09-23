package folder

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"gitlab.com/dynamo.foss/projekt/pkg/cli"
	"gitlab.com/dynamo.foss/projekt/pkg/folderutil"
	"gitlab.com/dynamo.foss/projekt/pkg/lazypath"
)

func NewFolderAddCmd(out io.Writer) *cobra.Command {
	o := &lazypath.Folder{}

	cmd := &cobra.Command{
		Use:     "add [folder path]",
		Short:   "Add your project folder to config",
		Args:    cobra.ExactArgs(1),
		Aliases: []string{"a"},
		RunE: func(cmd *cobra.Command, args []string) error {
			path, err := lazypath.NormalizePath(args[0])
			if err != nil {
				return err
			}
			// A relative path stored in the config would resolve differently on
			// every later run, so only absolute existing folders are accepted.
			info, err := os.Stat(path)
			if err != nil {
				return fmt.Errorf("cannot add %s: %w", path, err)
			}
			if !info.IsDir() {
				return fmt.Errorf("cannot add %s: not a directory", path)
			}
			if !o.IsWorkspace && o.RegexMatch != "" {
				return fmt.Errorf("--regex only works together with --as-workspace")
			}
			if o.IsWorkspace && o.Name != "" {
				// A workspace is never reached by a name of its own; only the
				// folders inside it are.
				return fmt.Errorf("--name cannot be used together with --as-workspace")
			}

			o.Path = path
			// Store the tags cleaned up, so that the config never carries a
			// blank or duplicated tag that could never be matched.
			o.Tags = lazypath.NormalizeTags(o.Tags)

			return folderutil.ImportFolderToConfig(o)
		},
	}

	f := cmd.Flags()
	f.StringVarP(&o.Name, "name", "n", "", "Short name of folder, defaults to the last element of its path. Only work with '-W false'")
	f.StringVarP(&o.Prefix, "prefix", "p", "", "Prefix of folder when call 'pj' or 'project folder go'")
	f.BoolVarP(&o.IsWorkspace, "as-workspace", "W", false, "Set folder as a workspace, like a parent folder of your projects")
	f.StringVarP(&o.RegexMatch, "regex", "R", "", "Go Regex match string to filter folder in workspace. Only work with '-W true'")
	f.Uint16VarP(&o.Priority, "priority", "P", 0, "Priority number of folder")
	f.StringSliceVarP(&o.Tags, "tags", "t", nil, "Tags to group this folder under, for filtering later")

	if err := cmd.RegisterFlagCompletionFunc("tags",
		func(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			return compListTags(toComplete)
		}); err != nil {
		cli.Warn("Cannot register completion for --tags: %v", err)
	}

	return cmd
}
