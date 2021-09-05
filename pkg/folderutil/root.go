package folderutil

import (
	"io"

	"github.com/gosuri/uitable"

	"gitlab.com/dynamo.foss/projekt/pkg/cli"
	"gitlab.com/dynamo.foss/projekt/pkg/lazypath"
)

func ImportFolderToConfig(f *lazypath.Folder) error {
	return f.AddToConfig()
}

func ListFolders(out io.Writer, isPlain bool) error {
	table := uitable.New()

	if isPlain {
		table.AddRow("PATH", "PREFIX", "IS WORKSPACE", "REGEX", "PRIORITY")
		for _, folder := range lazypath.GetConfig().Folders {
			table.AddRow(folder.Path, folder.Prefix, folder.IsWorkspace, folder.GetRegexMatch(), folder.Priority)
		}
	} else {
		table.AddRow("SHORT NAME", "PATH", "WORKSPACE PATH")
		folders, err := ParseConfig(lazypath.GetConfig())
		if err != nil {
			return err
		}
		for _, folder := range folders {
			table.AddRow(folder.ShortName, folder.Path, folder.Workspace)
		}
	}

	return cli.EncodeTable(out, table)
}
