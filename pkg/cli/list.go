package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/jedib0t/go-pretty/v6/table"
)

// OutputFormat selects how a listing is rendered.
type OutputFormat string

const (
	// OutputTable is the bordered, optionally coloured table meant for reading.
	OutputTable OutputFormat = "table"
	// OutputJSON is an array of objects, one per row.
	OutputJSON OutputFormat = "json"
	// OutputTSV is one row per line with tab-separated columns, meant for
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

// ListColumn is one column of a listing, under the label a table shows and the
// key a JSON object uses.
type ListColumn struct {
	Header string
	Key    string
}

// ListView is a rendering-independent listing: the columns, and one slice of
// values per row. Every command that lists something builds one of these and
// lets EncodeList decide what it looks like.
type ListView struct {
	Columns []ListColumn
	Rows    [][]any
}

// AppendRow adds one row of cells, in the order of the columns.
func (v *ListView) AppendRow(cells ...any) {
	v.Rows = append(v.Rows, cells)
}

// ListOutputOption is the rendering half of a listing command's options.
type ListOutputOption struct {
	NoHeaders bool
	NoColor   bool
	// Output selects the rendering. The empty value means OutputTable, so that
	// a zero option keeps the table behaviour.
	Output OutputFormat
}

// EncodeList writes a listing in the requested format.
func EncodeList(out io.Writer, v ListView, o ListOutputOption) error {
	switch o.Output {
	case OutputJSON:
		return encodeListJSON(out, v)
	case OutputTSV:
		return encodeListTSV(out, v, o.NoHeaders)
	case OutputTable, "":
		return encodeListTable(out, v, o)
	default:
		return fmt.Errorf("unknown output format %q, want one of: %s", o.Output, strings.Join(OutputFormats, ", "))
	}
}

func encodeListTable(out io.Writer, v ListView, o ListOutputOption) error {
	tw := table.NewWriter()

	if !o.NoHeaders {
		header := make(table.Row, 0, len(v.Columns))
		for _, col := range v.Columns {
			header = append(header, col.Header)
		}
		tw.AppendHeader(header)
	}
	for _, row := range v.Rows {
		cells := make(table.Row, 0, len(row))
		for _, cell := range row {
			cells = append(cells, displayValue(cell))
		}
		tw.AppendRow(cells)
	}

	return EncodeTable(out, tw, o.NoColor)
}

// displayValue renders a cell for the two text formats. A list of values reads
// as "go,work" rather than Go's default "[go work]".
func displayValue(cell any) string {
	if list, ok := cell.([]string); ok {
		return strings.Join(list, ",")
	}
	return fmt.Sprint(cell)
}

// encodeListTSV writes one row per line, so that a shell script can read the
// listing with `read` or `cut` instead of stripping table borders.
func encodeListTSV(out io.Writer, v ListView, noHeaders bool) error {
	var b strings.Builder

	if !noHeaders {
		headers := make([]string, 0, len(v.Columns))
		for _, col := range v.Columns {
			headers = append(headers, col.Header)
		}
		b.WriteString(strings.Join(headers, "\t"))
		b.WriteByte('\n')
	}
	for _, row := range v.Rows {
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

// encodeListJSON writes an array of objects, one per row, keyed by column. The
// array is always present, so a consumer never has to special-case an empty
// listing.
func encodeListJSON(out io.Writer, v ListView) error {
	objects := make([]map[string]any, 0, len(v.Rows))
	for _, row := range v.Rows {
		object := make(map[string]any, len(v.Columns))
		for i, col := range v.Columns {
			object[col.Key] = jsonValue(row[i])
		}
		objects = append(objects, object)
	}

	enc := json.NewEncoder(out)
	enc.SetIndent("", "  ")
	return enc.Encode(objects)
}

// jsonValue keeps an empty list as [] rather than null, for the same reason an
// empty listing is []: a consumer should only ever meet one shape.
func jsonValue(cell any) any {
	if list, ok := cell.([]string); ok && list == nil {
		return []string{}
	}
	return cell
}
