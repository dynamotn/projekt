package folderutil

import (
	"io"

	"github.com/gosuri/uitable"
	"github.com/fatih/color"

	"gitlab.com/dynamo.foss/projekt/pkg/cli"
	"gitlab.com/dynamo.foss/projekt/pkg/lazypath"
)

func ImportFolderToConfig(f *lazypath.Folder) error {
	return f.AddToConfig()
}

func ListFolders(out io.Writer, isPlain bool) error {
	table := uitable.New()
	green := color.New(color.FgGreen).SprintFunc()

	if isPlain {
		table.AddRow(green("PATH"), green("PREFIX"), green("REGEX"), green("PRIORITY"), green("IS WORKSPACE"))
		for _, folder := range lazypath.GetConfig().Folders {
			table.AddRow(folder.Path, folder.Prefix, folder.GetRegexMatch(), folder.Priority, folder.IsWorkspace)
		}
	} else {
		table.AddRow(green("SHORT NAME"), green("PATH"), green("WORKSPACE PATH"))
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

func RemoveFolderFromConfig(path string) error {
	return lazypath.RemoveFromConfig(path)
}
