package cli

import (
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
	tw.SetOutputMirror(out)
	tw.Render()
	return nil
}
