package folder

import (
	"sort"

	"github.com/spf13/cobra"

	"gitlab.com/dynamo-tools/projekt/pkg/cli"
	"gitlab.com/dynamo-tools/projekt/pkg/folderutil"
	"gitlab.com/dynamo-tools/projekt/pkg/lazypath"
)

func compListShortNames(_ string) ([]string, cobra.ShellCompDirective) {
	parsedFolders, err := folderutil.ParseConfig(lazypath.GetConfig())
	if err != nil {
		cli.Error("Can't parse config: %v", err)
		return nil, cobra.ShellCompDirectiveError
	}

	var result []string

	for _, pFolder := range parsedFolders {
		result = append(result, pFolder.ShortName)
	}

	return result, cobra.ShellCompDirectiveNoFileComp
}

// compListTags completes a --tags value with the tags already in the config, so
// that a typo shows up as a missing suggestion rather than an empty listing.
func compListTags(_ string) ([]string, cobra.ShellCompDirective) {
	seen := make(map[string]struct{})

	var result []string
	for _, folder := range lazypath.GetConfig().Folders {
		for _, tag := range folder.GetTags() {
			if _, dup := seen[tag]; dup {
				continue
			}
			seen[tag] = struct{}{}
			result = append(result, tag)
		}
	}
	sort.Strings(result)

	return result, cobra.ShellCompDirectiveNoFileComp
}

// registerTagsFlag adds the --tags filter, which every command that selects
// folders offers with the same meaning: keep the folders carrying all of them.
func registerTagsFlag(cmd *cobra.Command, target *[]string, action string) {
	cmd.Flags().StringSliceVarP(target, "tags", "t", nil,
		"Only "+action+" folders carrying all of these tags")

	if err := cmd.RegisterFlagCompletionFunc("tags",
		func(_ *cobra.Command, _ []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			return compListTags(toComplete)
		}); err != nil {
		cli.Warn("Cannot register completion for --tags: %v", err)
	}
}
