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
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/jedib0t/go-pretty/v6/table"

	"gitlab.com/dynamo.foss/projekt/pkg/cli"
	"gitlab.com/dynamo.foss/projekt/pkg/lazypath"
)

// OutputFormat selects how ListFolders renders its result.
type OutputFormat string

const (
	// OutputTable is the bordered, optionally coloured table meant for reading.
	OutputTable OutputFormat = "table"
	// OutputJSON is an array of objects, one per folder.
	OutputJSON OutputFormat = "json"
	// OutputTSV is one folder per line with tab-separated columns, meant for
	// shell scripts and completion, which should never have to parse a table.
	OutputTSV OutputFormat = "tsv"
)

// OutputFormats lists the accepted --output values, for validation and for
// shell completion.
var OutputFormats = []string{string(OutputTable), string(OutputJSON), string(OutputTSV)}

// ParseOutputFormat validates a --output value.
func ParseOutputFormat(s string) (OutputFormat, error) {
	switch OutputFormat(s) {
	case OutputTable, OutputJSON, OutputTSV:
		return OutputFormat(s), nil
	default:
		return "", fmt.Errorf("unknown output format %q, want one of: %s", s, strings.Join(OutputFormats, ", "))
	}
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

// listColumn is one column of a listing, under the label a table shows and the
// key a JSON object uses.
type listColumn struct {
	header string
	key    string
}

// listView is a rendered-format-independent listing: the columns, and one slice
// of values per folder.
type listView struct {
	columns []listColumn
	rows    [][]any
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

	switch o.Output {
	case OutputJSON:
		return view.encodeJSON(out)
	case OutputTSV:
		return view.encodeTSV(out, o.NoHeaders)
	case OutputTable, "":
		return view.encodeTable(out, o)
	default:
		return fmt.Errorf("unknown output format %q, want one of: %s", o.Output, strings.Join(OutputFormats, ", "))
	}
}

// buildListView collects what to list, without deciding how to render it.
func buildListView(o *ListOption) (listView, error) {
	if o.IsPlain {
		view := listView{columns: []listColumn{
			{header: "PATH", key: "path"},
			{header: "NAME", key: "name"},
			{header: "PREFIX", key: "prefix"},
			{header: "REGEX", key: "regex"},
			{header: "PRIORITY", key: "priority"},
			{header: "IS WORKSPACE", key: "isWorkspace"},
			{header: "TAGS", key: "tags"},
		}}
		for _, folder := range lazypath.GetConfig().Folders {
			if !lazypath.HasTags(folder.Tags, o.Tags) {
				continue
			}
			view.rows = append(view.rows, []any{
				folder.Path, folder.Name, folder.Prefix,
				folder.GetRegexMatch(), folder.Priority, folder.IsWorkspace,
				folder.GetTags(),
			})
		}
		return view, nil
	}

	folders, err := ParseConfig(lazypath.GetConfig())
	if err != nil {
		return listView{}, err
	}
	folders = FilterByTags(folders, o.Tags)

	if o.ShortOnly {
		view := listView{columns: []listColumn{{header: "SHORT NAME", key: "shortName"}}}
		for _, folder := range folders {
			view.rows = append(view.rows, []any{folder.ShortName})
		}
		return view, nil
	}

	view := listView{columns: []listColumn{
		{header: "SHORT NAME", key: "shortName"},
		{header: "PATH", key: "path"},
		{header: "WORKSPACE PATH", key: "workspace"},
		{header: "TAGS", key: "tags"},
	}}
	for _, folder := range folders {
		view.rows = append(view.rows, []any{folder.ShortName, folder.Path, folder.Workspace, folder.Tags})
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

func (v listView) encodeTable(out io.Writer, o *ListOption) error {
	tw := table.NewWriter()

	if !o.NoHeaders {
		header := make(table.Row, 0, len(v.columns))
		for _, col := range v.columns {
			header = append(header, col.header)
		}
		tw.AppendHeader(header)
	}
	for _, row := range v.rows {
		cells := make(table.Row, 0, len(row))
		for _, cell := range row {
			cells = append(cells, displayValue(cell))
		}
		tw.AppendRow(cells)
	}

	return cli.EncodeTable(out, tw, o.NoColor)
}

// displayValue renders a cell for the two text formats. A list of tags reads as
// "go,work" rather than Go's default "[go work]".
func displayValue(cell any) string {
	if list, ok := cell.([]string); ok {
		return strings.Join(list, ",")
	}
	return fmt.Sprint(cell)
}

// encodeTSV writes one folder per line, so that a shell script can read the
// listing with `read` or `cut` instead of stripping table borders.
func (v listView) encodeTSV(out io.Writer, noHeaders bool) error {
	var b strings.Builder

	if !noHeaders {
		headers := make([]string, 0, len(v.columns))
		for _, col := range v.columns {
			headers = append(headers, col.header)
		}
		b.WriteString(strings.Join(headers, "\t"))
		b.WriteByte('\n')
	}
	for _, row := range v.rows {
		cells := make([]string, 0, len(row))
		for _, cell := range row {
			cells = append(cells, displayValue(cell))
		}
		b.WriteString(strings.Join(cells, "\t"))
		b.WriteByte('\n')
	}

	_, err := io.WriteString(out, b.String())
	return err
}

// encodeJSON writes an array of objects, one per folder, keyed by column. The
// array is always present, so a consumer never has to special-case no folders.
func (v listView) encodeJSON(out io.Writer) error {
	objects := make([]map[string]any, 0, len(v.rows))
	for _, row := range v.rows {
		object := make(map[string]any, len(v.columns))
		for i, col := range v.columns {
			object[col.key] = jsonValue(row[i])
		}
		objects = append(objects, object)
	}

	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	return enc.Encode(objects)
}

// jsonValue keeps an untagged folder's tags as [] rather than null, for the
// same reason an empty listing is []: a consumer should only meet one shape.
func jsonValue(cell any) any {
	if list, ok := cell.([]string); ok && list == nil {
		return []string{}
	}
	return cell
}

// RemoveFolderFromConfig removes a folder from the configuration by path
func RemoveFolderFromConfig(path string) error {
	return lazypath.RemoveFromConfig(path)
}
