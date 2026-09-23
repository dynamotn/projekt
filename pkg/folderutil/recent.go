package folderutil

import (
	"fmt"
	"io"
	"time"

	"gitlab.com/dynamo.foss/projekt/pkg/cli"
	"gitlab.com/dynamo.foss/projekt/pkg/lazypath"
)

// RecentOptions drives `projekt folder recent`.
type RecentOptions struct {
	// Limit is how many projects to show. Zero means all of them.
	Limit int
	// NamesOnly keeps the name column alone, the listing a script wants.
	NamesOnly bool
	cli.ListOutputOption
}

// ListRecent displays the projects in the order they were last jumped to.
func ListRecent(out io.Writer, o *RecentOptions) error {
	visits := lazypath.ReadHistory()
	if o.Limit > 0 && len(visits) > o.Limit {
		visits = visits[:o.Limit]
	}

	if len(visits) == 0 && o.Output != cli.OutputJSON {
		cli.Warn("Nothing jumped to yet")
		return nil
	}

	// Most recent first is the whole point, so the table must not re-sort it.
	view := cli.ListView{Ordered: true, Columns: []cli.ListColumn{
		{Header: "NAME", Key: "name"},
		{Header: "PATH", Key: "path"},
		{Header: "VISITS", Key: "visits"},
		{Header: "LAST", Key: "last"},
	}}
	if o.NamesOnly {
		view.Columns = view.Columns[:1]
	}

	for _, visit := range visits {
		if o.NamesOnly {
			view.AppendRow(visit.Name)
			continue
		}
		view.AppendRow(visit.Name, visit.Path, visit.Count, Ago(visit.At))
	}

	return cli.EncodeList(out, view, o.ListOutputOption)
}

// Ago says how long ago something was, in as few words as it takes.
func Ago(at time.Time) string {
	elapsed := time.Since(at)
	switch {
	case elapsed < time.Minute:
		return "just now"
	case elapsed < time.Hour:
		return plural(int(elapsed.Minutes()), "minute")
	case elapsed < 24*time.Hour:
		return plural(int(elapsed.Hours()), "hour")
	case elapsed < 365*24*time.Hour:
		return plural(int(elapsed.Hours()/24), "day")
	default:
		return plural(int(elapsed.Hours()/24/365), "year")
	}
}

func plural(count int, unit string) string {
	if count == 1 {
		return fmt.Sprintf("1 %s ago", unit)
	}
	return fmt.Sprintf("%d %ss ago", count, unit)
}
