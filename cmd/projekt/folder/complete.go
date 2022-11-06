package folder

import (
	"github.com/spf13/cobra"

	"gitlab.com/dynamo.foss/projekt/pkg/cli"
	"gitlab.com/dynamo.foss/projekt/pkg/folderutil"
	"gitlab.com/dynamo.foss/projekt/pkg/lazypath"
)

func compListShortNames(search string) ([]string, cobra.ShellCompDirective) {
	parsedFolders, err := folderutil.ParseConfig(lazypath.GetConfig())
	if err != nil {
		cli.Error("Can't parse config", err)
		return nil, cobra.ShellCompDirectiveError
	}

	var result []string

	for _, pFolder := range parsedFolders {
		result = append(result, pFolder.ShortName)
	}

	return result, cobra.ShellCompDirectiveNoFileComp
}
