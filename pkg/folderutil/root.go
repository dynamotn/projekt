// Package folderutil provides utilities for managing and parsing project folders.
//
// It offers functionality to parse folder configurations, find folders by short names,
// and manage folder lists with support for workspaces and regex matching.
//
// Example usage:
//
//	config := lazypath.GetConfig()
//	folders, err := folderutil.ParseConfig(config)
//	if err != nil {
//	    log.Fatal(err)
//	}
//
//	for _, folder := range folders {
//	    fmt.Printf("%s -> %s\n", folder.ShortName, folder.Path)
//	}
package folderutil

import (
	"io"

	"github.com/jedib0t/go-pretty/v6/table"

	"gitlab.com/dynamo.foss/projekt/pkg/cli"
	"gitlab.com/dynamo.foss/projekt/pkg/lazypath"
)

// ListOption contains options for listing folders
type ListOption struct {
	IsPlain   bool
	ShortOnly bool
	NoHeaders bool
	NoColor   bool
}

// ImportFolderToConfig adds a folder to the configuration
func ImportFolderToConfig(f *lazypath.Folder) error {
	return f.AddToConfig()
}

// ListFolders displays a list of configured folders in a table format
func ListFolders(out io.Writer, o *ListOption) error {
	tw := table.NewWriter()

	if o.IsPlain {
		if !o.NoHeaders {
			tw.AppendHeader(table.Row{"PATH", "PREFIX", "REGEX", "PRIORITY", "IS WORKSPACE"})
		}
		for _, folder := range lazypath.GetConfig().Folders {
			tw.AppendRow(table.Row{folder.Path, folder.Prefix, folder.GetRegexMatch(), folder.Priority, folder.IsWorkspace})
		}
	} else {
		if !o.NoHeaders {
			if o.ShortOnly {
				tw.AppendHeader(table.Row{"SHORT NAME"})
			} else {
				tw.AppendHeader(table.Row{"SHORT NAME", "PATH", "WORKSPACE PATH"})
			}
		}
		folders, err := ParseConfig(lazypath.GetConfig())
		if err != nil {
			return err
		}
		for _, folder := range folders {
			if o.ShortOnly {
				tw.AppendRow(table.Row{folder.ShortName})
			} else {
				tw.AppendRow(table.Row{folder.ShortName, folder.Path, folder.Workspace})
			}
		}
	}

	return cli.EncodeTable(out, tw, o.NoColor)
}

// RemoveFolderFromConfig removes a folder from the configuration by path
func RemoveFolderFromConfig(path string) error {
	return lazypath.RemoveFromConfig(path)
}
