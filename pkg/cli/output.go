package cli

import (
	"fmt"
	"io"

	"github.com/jedib0t/go-pretty/v6/table"
)

// EncodeTable writes a table to the output writer, sorted by its first column.
func EncodeTable(out io.Writer, tw table.Writer, noColor bool) error {
	return encodeTable(out, tw, noColor, true)
}

// encodeTable writes a table, sorting it by its first column only when the
// rows have no order of their own. A listing that is already in a meaningful
// order — most recent first, say — would lose the point of it.
func encodeTable(out io.Writer, tw table.Writer, noColor, sorted bool) error {
	if noColor {
		tw.SetStyle(table.StyleDefault)
	} else {
		tw.SetStyle(table.StyleColoredBlackOnCyanWhite)
	}
	if sorted {
		tw.SortBy([]table.SortBy{{Number: 1, Mode: table.Asc}})
	}
	tw.SetIndexColumn(1)

	// Render to a string instead of a mirror so that a failing writer
	// (a closed pipe, a full disk) is reported to the caller.
	_, err := fmt.Fprintln(out, tw.Render())
	return err
}
