package cli

import (
	"fmt"
	"io"
	"os"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/mattn/go-isatty"
)

// EncodeTable writes a table to the output writer, sorted by its first column.
func EncodeTable(out io.Writer, tw table.Writer, noColor bool) error {
	return encodeTable(out, tw, noColor, true)
}

// encodeTable writes a table, sorting it by its first column only when the
// rows have no order of their own. A listing that is already in a meaningful
// order — most recent first, say — would lose the point of it.
func encodeTable(out io.Writer, tw table.Writer, noColor, sorted bool) error {
	if noColor || !wantsColor(out) {
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

// wantsColor reports whether out is a terminal that should get color: never a
// pipe, a file or a buffer, where the escapes would be read as text, and
// never with NO_COLOR set.
func wantsColor(out io.Writer) bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}
	f, ok := out.(*os.File)
	return ok && (isatty.IsTerminal(f.Fd()) || isatty.IsCygwinTerminal(f.Fd()))
}
