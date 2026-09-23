package cli

import (
	"fmt"
	"io"

	"github.com/jedib0t/go-pretty/v6/table"
)

// EncodeTable writes a table to the output writer
func EncodeTable(out io.Writer, tw table.Writer, noColor bool) error {
	if noColor {
		tw.SetStyle(table.StyleDefault)
	} else {
		tw.SetStyle(table.StyleColoredBlackOnCyanWhite)
	}
	tw.SortBy([]table.SortBy{{Number: 1, Mode: table.Asc}})
	tw.SetIndexColumn(1)

	// Render to a string instead of a mirror so that a failing writer
	// (a closed pipe, a full disk) is reported to the caller.
	_, err := fmt.Fprintln(out, tw.Render())
	return err
}
