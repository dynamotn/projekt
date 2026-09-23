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

	"gitlab.com/dynamo.foss/projekt/pkg/cli"
	"gitlab.com/dynamo.foss/projekt/pkg/lazypath"
)

// The listing formats live in the cli package, so that every command that
// lists something renders it the same way. They are kept here under their
// original names for the callers that already use them.
type OutputFormat = cli.OutputFormat

const (
	// OutputTable is the bordered, optionally coloured table meant for reading.
	OutputTable = cli.OutputTable
	// OutputJSON is an array of objects, one per folder.
	OutputJSON = cli.OutputJSON
	// OutputTSV is one folder per line with tab-separated columns.
	OutputTSV = cli.OutputTSV
)

// OutputFormats lists the accepted --output values, for validation and for
// shell completion.
var OutputFormats = cli.OutputFormats

// ParseOutputFormat validates a --output value.
func ParseOutputFormat(s string) (OutputFormat, error) {
	return cli.ParseOutputFormat(s)
}

// ListOption contains options for listing folders
type ListOption struct {
	IsPlain   bool
	ShortOnly bool
	NoHeaders bool
	NoColor   bool
	// Output selects the rendering. The empty value means OutputTable, so that
	// a zero ListOption keeps the original behaviour.
	Output OutputFormat
	// Tags keeps only the folders carrying every one of these tags. Empty lists
	// everything.
	Tags []string
}

// ImportFolderToConfig adds a folder to the configuration
func ImportFolderToConfig(f *lazypath.Folder) error {
	return f.AddToConfig()
}

// ListFolders displays a list of configured folders in the requested format.
func ListFolders(out io.Writer, o *ListOption) error {
	view, err := buildListView(o)
	if err != nil {
		return err
	}

	return cli.EncodeList(out, view, cli.ListOutputOption{
		NoHeaders: o.NoHeaders,
		NoColor:   o.NoColor,
		Output:    o.Output,
	})
}

// buildListView collects what to list, without deciding how to render it.
func buildListView(o *ListOption) (cli.ListView, error) {
	if o.IsPlain {
		view := cli.ListView{Columns: []cli.ListColumn{
			{Header: "PATH", Key: "path"},
			{Header: "NAME", Key: "name"},
			{Header: "PREFIX", Key: "prefix"},
			{Header: "REGEX", Key: "regex"},
			{Header: "PRIORITY", Key: "priority"},
			{Header: "IS WORKSPACE", Key: "isWorkspace"},
			{Header: "TAGS", Key: "tags"},
		}}
		for _, folder := range lazypath.GetConfig().Folders {
			if !lazypath.HasTags(folder.Tags, o.Tags) {
				continue
			}
			view.AppendRow(
				folder.Path, folder.Name, folder.Prefix,
				folder.GetRegexMatch(), folder.Priority, folder.IsWorkspace,
				folder.GetTags(),
			)
		}
		return view, nil
	}

	folders, err := ParseConfig(lazypath.GetConfig())
	if err != nil {
		return cli.ListView{}, err
	}
	folders = FilterByTags(folders, o.Tags)

	if o.ShortOnly {
		view := cli.ListView{Columns: []cli.ListColumn{{Header: "SHORT NAME", Key: "shortName"}}}
		for _, folder := range folders {
			view.AppendRow(folder.ShortName)
		}
		return view, nil
	}

	view := cli.ListView{Columns: []cli.ListColumn{
		{Header: "SHORT NAME", Key: "shortName"},
		{Header: "PATH", Key: "path"},
		{Header: "WORKSPACE PATH", Key: "workspace"},
		{Header: "TAGS", Key: "tags"},
	}}
	for _, folder := range folders {
		view.AppendRow(folder.ShortName, folder.Path, folder.Workspace, folder.Tags)
	}
	return view, nil
}

// FilterByTags keeps the folders carrying every one of the wanted tags.
func FilterByTags(folders []ParsedFolder, tags []string) []ParsedFolder {
	if len(lazypath.NormalizeTags(tags)) == 0 {
		return folders
	}

	result := make([]ParsedFolder, 0, len(folders))
	for _, folder := range folders {
		if lazypath.HasTags(folder.Tags, tags) {
			result = append(result, folder)
		}
	}
	return result
}

// RemoveFolderFromConfig removes a folder from the configuration by path
func RemoveFolderFromConfig(path string) error {
	return lazypath.RemoveFromConfig(path)
}
