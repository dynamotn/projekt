package tplutil

import (
	"fmt"
	"strings"
)

// diffLimit is the point past which a file is reported as changed rather than
// diffed line by line. The comparison is quadratic, and nobody reads a
// thousand-line hunk anyway.
const diffLimit = 5000

// UnifiedDiff describes the change from before to after, in the format `git
// diff` and `patch` both speak.
//
// An empty string means the two are the same.
func UnifiedDiff(path string, before, after []byte, context int) string {
	if string(before) == string(after) {
		return ""
	}
	if context < 0 {
		context = 3
	}

	header := fmt.Sprintf("--- a/%s\n+++ b/%s\n", path, path)

	oldLines, newLines := splitLines(before), splitLines(after)
	if len(oldLines) > diffLimit || len(newLines) > diffLimit {
		// Saying nothing would read as "unchanged", which is the one thing it
		// is not.
		return header + fmt.Sprintf("@@ %d lines -> %d lines @@\n", len(oldLines), len(newLines))
	}

	hunks := hunksOf(oldLines, newLines, context)
	if hunks == "" {
		return ""
	}
	return header + hunks
}

// splitLines cuts content into lines, keeping track of a missing final
// newline the way a diff must.
func splitLines(content []byte) []string {
	if len(content) == 0 {
		return nil
	}
	text := string(content)
	trailing := strings.HasSuffix(text, "\n")
	if trailing {
		text = strings.TrimSuffix(text, "\n")
	}
	lines := strings.Split(text, "\n")
	if !trailing && len(lines) > 0 {
		lines[len(lines)-1] += "\n\\ No newline at end of file"
	}
	return lines
}

// edit is one line of the comparison.
type edit struct {
	// sign is ' ', '-' or '+'.
	sign byte
	text string
}

// hunksOf renders the changed regions with their surrounding context.
//
// Two changes close enough that their context would touch belong to one hunk,
// which is the rule diff(1) follows: a gap of more than twice the context is
// what separates them.
func hunksOf(oldLines, newLines []string, context int) string {
	edits := editScript(oldLines, newLines)

	var changed []int
	for i, e := range edits {
		if e.sign != ' ' {
			changed = append(changed, i)
		}
	}
	if len(changed) == 0 {
		return ""
	}

	// Where each edit starts, on each side, so a hunk header can be written
	// without walking the script again.
	oldAt := make([]int, len(edits)+1)
	newAt := make([]int, len(edits)+1)
	oldAt[0], newAt[0] = 1, 1
	for i, e := range edits {
		oldAt[i+1], newAt[i+1] = oldAt[i], newAt[i]
		if e.sign != '+' {
			oldAt[i+1]++
		}
		if e.sign != '-' {
			newAt[i+1]++
		}
	}

	var out strings.Builder
	for g := 0; g < len(changed); {
		last := g
		for last+1 < len(changed) && changed[last+1]-changed[last]-1 <= 2*context {
			last++
		}
		first, final := changed[g], changed[last]

		from := max(0, first-context)
		to := min(len(edits), final+context+1)

		var oldCount, newCount int
		for _, e := range edits[from:to] {
			if e.sign != '+' {
				oldCount++
			}
			if e.sign != '-' {
				newCount++
			}
		}
		fmt.Fprintf(&out, "@@ -%s +%s @@\n", span(oldAt[from], oldCount), span(newAt[from], newCount))
		for _, e := range edits[from:to] {
			fmt.Fprintf(&out, "%c%s\n", e.sign, e.text)
		}
		g = last + 1
	}
	return out.String()
}

// span is the `start,count` of a hunk header, which drops the count when it is
// one and the start when the side is empty.
func span(start, count int) string {
	if count == 0 {
		return fmt.Sprintf("%d,0", start-1)
	}
	if count == 1 {
		return fmt.Sprintf("%d", start)
	}
	return fmt.Sprintf("%d,%d", start, count)
}

// editScript is the shortest way to read the old lines as the new ones, found
// through the longest common subsequence of the two.
func editScript(oldLines, newLines []string) []edit {
	var edits []edit
	lcsWalk(oldLines, newLines, func(sign byte, i, j int) {
		if sign == '+' {
			edits = append(edits, edit{sign, newLines[j]})
			return
		}
		edits = append(edits, edit{sign, oldLines[i]})
	})
	return edits
}

// lcsWalk walks a and b along their longest common subsequence, calling step
// in order with ' ' for a line both have, '-' for one only a has and '+' for
// one only b has, and the index of the line in each.
//
// The lines the two share at the start and at the end are matched before the
// table is built, so it is the size of what changed rather than of the whole
// file: a one-line change to a 5000-line file is not a 200 MB table.
func lcsWalk(a, b []string, step func(sign byte, i, j int)) {
	head := 0
	for head < len(a) && head < len(b) && a[head] == b[head] {
		step(' ', head, head)
		head++
	}
	tail := 0
	for tail < len(a)-head && tail < len(b)-head && a[len(a)-1-tail] == b[len(b)-1-tail] {
		tail++
	}
	ma, mb := a[head:len(a)-tail], b[head:len(b)-tail]

	lengths := make([][]int, len(ma)+1)
	for i := range lengths {
		lengths[i] = make([]int, len(mb)+1)
	}
	for i := len(ma) - 1; i >= 0; i-- {
		for j := len(mb) - 1; j >= 0; j-- {
			if ma[i] == mb[j] {
				lengths[i][j] = lengths[i+1][j+1] + 1
				continue
			}
			lengths[i][j] = max(lengths[i+1][j], lengths[i][j+1])
		}
	}

	i, j := 0, 0
	for i < len(ma) && j < len(mb) {
		switch {
		case ma[i] == mb[j]:
			step(' ', head+i, head+j)
			i++
			j++
		case lengths[i+1][j] >= lengths[i][j+1]:
			step('-', head+i, head+j)
			i++
		default:
			step('+', head+i, head+j)
			j++
		}
	}
	for ; i < len(ma); i++ {
		step('-', head+i, head+j)
	}
	for ; j < len(mb); j++ {
		step('+', head+i, head+j)
	}
	for k := tail; k > 0; k-- {
		step(' ', len(a)-k, len(b)-k)
	}
}
