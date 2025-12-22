package folderutil

import (
	"io"

	"github.com/fatih/color"
	"github.com/gosuri/uitable"

	"gitlab.com/dynamo.foss/projekt/pkg/cli"
	"gitlab.com/dynamo.foss/projekt/pkg/lazypath"
)

type ListOption struct {
	IsPlain   bool
	ShortOnly bool
	NoHeaders bool
}

func ImportFolderToConfig(f *lazypath.Folder) error {
	return f.AddToConfig()
}

func ListFolders(out io.Writer, o *ListOption) error {
	table := uitable.New()
	green := color.New(color.FgGreen).SprintFunc()

	if o.IsPlain {
		if !o.NoHeaders {
			table.AddRow(green("PATH"), green("PREFIX"), green("REGEX"), green("PRIORITY"), green("IS WORKSPACE"))
		}
		for _, folder := range lazypath.GetConfig().Folders {
			table.AddRow(folder.Path, folder.Prefix, folder.GetRegexMatch(), folder.Priority, folder.IsWorkspace)
		}
	} else {
		if !o.NoHeaders {
			if o.ShortOnly {
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
			if o.ShortOnly {
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
