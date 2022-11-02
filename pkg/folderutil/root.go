package folderutil

import (
	"io"

	"github.com/fatih/color"
	"github.com/gosuri/uitable"

	"gitlab.com/dynamo.foss/projekt/pkg/cli"
	"gitlab.com/dynamo.foss/projekt/pkg/lazypath"
)

func ImportFolderToConfig(f *lazypath.Folder) error {
	return f.AddToConfig()
}

func ListFolders(out io.Writer, isPlain bool, showOnlyShortName bool, noHeaders bool) error {
	table := uitable.New()
	green := color.New(color.FgGreen).SprintFunc()

	if isPlain {
		if !noHeaders {
			table.AddRow(green("PATH"), green("PREFIX"), green("REGEX"), green("PRIORITY"), green("IS WORKSPACE"))
		}
		for _, folder := range lazypath.GetConfig().Folders {
			table.AddRow(folder.Path, folder.Prefix, folder.GetRegexMatch(), folder.Priority, folder.IsWorkspace)
		}
	} else {
		if !noHeaders {
			if showOnlyShortName {
				table.AddRow(green("SHORT NAME"))
			} else {
				table.AddRow(green("SHORT NAME"), green("PATH"), green("WORKSPACE PATH"))
			}
		}
		folders, err := ParseConfig(lazypath.GetConfig())
		if err != nil {
			return err
		}
		for _, folder := range folders {
			if showOnlyShortName {
				table.AddRow(folder.ShortName)
			} else {
				table.AddRow(folder.ShortName, folder.Path, folder.Workspace)
			}
		}
	}

	return cli.EncodeTable(out, table)
}

func RemoveFolderFromConfig(path string) error {
	return lazypath.RemoveFromConfig(path)
}
