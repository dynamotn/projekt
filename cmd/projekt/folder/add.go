package folder

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"gitlab.com/dynamo-tools/projekt/pkg/cli"
	"gitlab.com/dynamo-tools/projekt/pkg/folderutil"
	"gitlab.com/dynamo-tools/projekt/pkg/lazypath"
)

func NewFolderAddCmd(out io.Writer) *cobra.Command {
	o := &lazypath.Folder{}
	discover := false
	fromRemote := false
	remote := folderutil.RemoteDiscoverOptions{}

	cmd := &cobra.Command{
		Use:     "add [folder path]",
		Short:   "Add your project folder to config",
		Args:    cobra.ExactArgs(1),
		Aliases: []string{"a"},
		Long: `Add your project folder to config.

With --discover, the repositories already cloned inside a workspace are read
off disk and written into its git section, instead of being typed out by hand.
Each one's origin remote decides which configured git server it belongs to.

With --from-remote, the git server itself is asked instead, which is what a
new machine needs: nothing is on disk yet. --host names a configured server
and --group the group or organisation on it; 'folder sync' then clones the
lot.

  projekt folder add ~/work/myorg -W -p myorg --from-remote --host gitlab --group myorg/team
  projekt folder add ~/work/myorg -W -p myorg --from-remote --host gitlab --group myorg --recursive
  projekt folder add ~/oss -W -p oss --from-remote --host github --group jane --exclude '^scratch'

A private group needs a token: GITLAB_TOKEN or GITHUB_TOKEN in the
environment, or a 'glab auth login' / 'gh auth login' already done. It is used
for the request and kept nowhere.`,
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
			if !o.IsWorkspace && (discover || fromRemote) {
				// Discovery fills the folders inside a workspace; a plain
				// folder has no children to scan or to clone.
				return fmt.Errorf("--discover and --from-remote only work together with --as-workspace")
			}
			if discover && fromRemote {
				return fmt.Errorf("--discover reads the disk and --from-remote asks the server; pick one")
			}
			if !fromRemote && (remote.Host != "" || remote.Group != "" || remote.Recursive || remote.Archived || remote.Include != "" || remote.Exclude != "") {
				return fmt.Errorf("--host, --group, --recursive, --archived, --include and --exclude go with --from-remote")
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

			if discover {
				git, err := folderutil.DiscoverRepos(*o, lazypath.GetConfig().GitServers)
				if err != nil {
					return fmt.Errorf("cannot discover repositories: %w", err)
				}
				o.Git = git
			}
			if fromRemote {
				git, err := folderutil.DiscoverRemoteRepos(lazypath.GetConfig().GitServers, remote)
				if err != nil {
					return fmt.Errorf("cannot list repositories: %w", err)
				}
				o.Git = git
			}

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
	f.BoolVarP(&discover, "discover", "D", false, "Read the repositories already cloned inside the workspace into its git config. Only works with '-W true'")
	f.BoolVar(&fromRemote, "from-remote", false, "Ask the git server which repositories --group has, and write them into the git config. Only works with '-W true'")
	f.StringVar(&remote.Host, "host", "", "Configured git server to ask, with --from-remote")
	f.StringVar(&remote.Group, "group", "", "Group or organisation to list, with --from-remote")
	f.BoolVar(&remote.Recursive, "recursive", false, "Include the repositories of subgroups (GitLab), with --from-remote")
	f.BoolVar(&remote.Archived, "archived", false, "Include archived repositories, with --from-remote")
	f.StringVar(&remote.Include, "include", "", "Keep only the repositories whose name matches this regex, with --from-remote")
	f.StringVar(&remote.Exclude, "exclude", "", "Drop the repositories whose name matches this regex, with --from-remote")

	if err := cmd.RegisterFlagCompletionFunc("host",
		func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
			var names []string
			for _, server := range lazypath.GetConfig().GitServers {
				names = append(names, server.Name)
			}
			return names, cobra.ShellCompDirectiveNoFileComp
		}); err != nil {
		cli.Warn("Cannot register completion for --host: %v", err)
	}

	if err := cmd.RegisterFlagCompletionFunc("tags",
		func(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			return compListTags(toComplete)
		}); err != nil {
		cli.Warn("Cannot register completion for --tags: %v", err)
	}

	return cmd
}
