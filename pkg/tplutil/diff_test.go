package tplutil

import (
	"math/rand"
	"os/exec"
	"strconv"
	"strings"
	"testing"
)

func TestUnifiedDiff_MatchesDiffU(t *testing.T) {
	cases := []struct {
		name          string
		before, after string
	}{
		{"one line changed", "a\nb\nc\nd\ne\nf\ng\n", "a\nb\nC\nd\ne\nf\ng\n"},
		{"appended", "a\nb\n", "a\nb\nc\n"},
		{"removed", "a\nb\nc\n", "a\nc\n"},
		{"two separate hunks", "1\n2\n3\n4\n5\n6\n7\n8\n9\n", "1\nX\n3\n4\n5\n6\n7\nY\n9\n"},
		{"emptied", "a\nb\n", ""},
		{"filled", "", "a\nb\n"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := UnifiedDiff("f", []byte(c.before), []byte(c.after), 3)
			want := diffU(t, c.before, c.after)
			if got != want {
				t.Errorf("UnifiedDiff() =\n%s\nwant\n%s", got, want)
			}
		})
	}
}

// diffU is what the system's diff makes of the same pair, which is the only
// definition of "unified diff" worth testing against.
func diffU(t *testing.T, before, after string) string {
	t.Helper()
	if _, err := exec.LookPath("diff"); err != nil {
		t.Skip("no diff(1) to compare against")
	}

	dir := t.TempDir()
	a, b := dir+"/a", dir+"/b"
	writeTemplate(t, a, before)
	writeTemplate(t, b, after)

	out, err := exec.Command("diff", "-U3", "--label", "a/f", "--label", "b/f", a, b).Output()
	if err != nil {
		if exit, ok := err.(*exec.ExitError); !ok || exit.ExitCode() != 1 {
			t.Fatalf("diff error = %v", err)
		}
	}
	return string(out)
}

// TestUnifiedDiff_RoundTripsOnRandomInput is the real test of a hand-written
// differ.
//
// Byte-identity with diff(1) is not the property worth holding: several edit
// scripts of the same length describe the same change, and GNU picks among
// them with heuristics of its own. What must hold is that the diff *is* the
// change — applying it to the old file gives the new one — and that it is no
// longer than the one diff(1) found.
func TestUnifiedDiff_RoundTripsOnRandomInput(t *testing.T) {
	random := rand.New(rand.NewSource(1))
	alphabet := []string{"a", "b", "c", "d", "e"}

	for i := 0; i < 500; i++ {
		before := randomLines(random, alphabet)
		after := randomLines(random, alphabet)

		got := UnifiedDiff("f", []byte(before), []byte(after), 3)
		if rebuilt := patch(t, before, got); rebuilt != after {
			t.Fatalf("UnifiedDiff(%q, %q) =\n%s\napplied gives %q", before, after, got, rebuilt)
		}
		if mine, theirs := edits(got), edits(diffU(t, before, after)); mine > theirs {
			t.Fatalf("UnifiedDiff(%q, %q) took %d edits, diff(1) took %d:\n%s", before, after, mine, theirs, got)
		}
	}
}

// patch rebuilds the new file from the old one and a unified diff, which is
// the only thing a diff is really for.
func patch(t *testing.T, before, diff string) string {
	t.Helper()
	if diff == "" {
		return before
	}

	old := splitLines([]byte(before))
	var out []string
	at := 0

	for _, line := range strings.Split(diff, "\n") {
		switch {
		case line == "" || strings.HasPrefix(line, "---") || strings.HasPrefix(line, "+++"):
			continue
		case strings.HasPrefix(line, "@@"):
			// Catch up on everything the hunk skipped over.
			start := hunkStart(t, line)
			for at < start-1 {
				out = append(out, old[at])
				at++
			}
		case strings.HasPrefix(line, " "):
			out = append(out, line[1:])
			at++
		case strings.HasPrefix(line, "-"):
			at++
		case strings.HasPrefix(line, "+"):
			out = append(out, line[1:])
		}
	}
	for at < len(old) {
		out = append(out, old[at])
		at++
	}

	if len(out) == 0 {
		return ""
	}
	return strings.Join(out, "\n") + "\n"
}

// hunkStart reads the old-side line number out of a hunk header.
func hunkStart(t *testing.T, header string) int {
	t.Helper()
	fields := strings.Fields(header)
	if len(fields) < 2 {
		t.Fatalf("hunk header %q has no old side", header)
	}
	span := strings.TrimPrefix(fields[1], "-")
	span, _, _ = strings.Cut(span, ",")
	start, err := strconv.Atoi(span)
	if err != nil {
		t.Fatalf("hunk header %q: %v", header, err)
	}
	if start == 0 {
		return 1
	}
	return start
}

// edits counts the lines a diff adds and removes, which is its length.
func edits(diff string) int {
	count := 0
	for _, line := range strings.Split(diff, "\n") {
		if strings.HasPrefix(line, "+++") || strings.HasPrefix(line, "---") {
			continue
		}
		if strings.HasPrefix(line, "+") || strings.HasPrefix(line, "-") {
			count++
		}
	}
	return count
}

func randomLines(random *rand.Rand, alphabet []string) string {
	var out strings.Builder
	for n := random.Intn(12); n > 0; n-- {
		out.WriteString(alphabet[random.Intn(len(alphabet))])
		out.WriteString("\n")
	}
	return out.String()
}

func TestUnifiedDiff_SameContentIsEmpty(t *testing.T) {
	if got := UnifiedDiff("f", []byte("a\n"), []byte("a\n"), 3); got != "" {
		t.Errorf("UnifiedDiff() = %q, want nothing for two identical files", got)
	}
}

func TestUnifiedDiff_MissingFinalNewline(t *testing.T) {
	got := UnifiedDiff("f", []byte("a"), []byte("a\n"), 3)
	if !strings.Contains(got, "No newline at end of file") {
		t.Errorf("UnifiedDiff() = %q, want the missing newline reported", got)
	}
}
