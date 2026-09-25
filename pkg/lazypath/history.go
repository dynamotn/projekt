package lazypath

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"gitlab.com/dynamo-tools/projekt/pkg/cli"
)

// PreviousName is what `pj -` passes: take me back where I was.
const PreviousName = "-"

// HistoryLimit is how many projects the history remembers. It is a jump list,
// not an archive, and an unbounded file would be read on every jump.
const HistoryLimit = 200

// HistoryFileOverride points the history somewhere else. Tests set it; nothing
// else needs to.
var HistoryFileOverride string

// Visit is one project, and when it was last jumped to.
type Visit struct {
	// Name is the short name it was reached by.
	Name string
	// Path is where it was at the time.
	Path string
	// Count is how many times it has been jumped to.
	Count int
	// At is when the last jump was.
	At time.Time
}

// StateHome returns $XDG_STATE_HOME, or the default the spec gives for it.
//
// The history belongs there rather than in the data or the cache directory: it
// should survive a reboot, and losing it should cost nothing but the order of
// a listing.
func StateHome() string {
	if state := strings.TrimSpace(os.Getenv("XDG_STATE_HOME")); state != "" {
		return state
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ".local/state"
	}
	return filepath.Join(home, ".local", "state")
}

// HistoryFile returns the file the jump history is kept in.
func HistoryFile() string {
	if HistoryFileOverride != "" {
		return HistoryFileOverride
	}
	return filepath.Join(StateHome(), "projekt", "history.tsv")
}

// ReadHistory returns the visited projects, most recently visited first.
//
// A history that cannot be read is not an error worth stopping a jump for: it
// comes back empty, and the next jump writes a fresh one.
func ReadHistory() []Visit {
	file, err := os.Open(HistoryFile())
	if err != nil {
		if !os.IsNotExist(err) {
			cli.Debug("Cannot read the jump history: %v", err)
		}
		return nil
	}
	defer file.Close()

	var visits []Visit
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		visit, ok := parseVisit(scanner.Text())
		if !ok {
			continue
		}
		visits = append(visits, visit)
	}
	if err := scanner.Err(); err != nil {
		cli.Debug("Cannot read the jump history: %v", err)
	}

	sortVisits(visits)
	return visits
}

// parseVisit reads one line: when, how often, the name and the path.
func parseVisit(line string) (Visit, bool) {
	fields := strings.Split(line, "\t")
	if len(fields) != 4 {
		return Visit{}, false
	}

	seconds, err := strconv.ParseInt(fields[0], 10, 64)
	if err != nil {
		return Visit{}, false
	}
	count, err := strconv.Atoi(fields[1])
	if err != nil {
		return Visit{}, false
	}
	if fields[2] == "" || fields[3] == "" {
		return Visit{}, false
	}

	return Visit{
		At:    time.Unix(seconds, 0),
		Count: count,
		Name:  fields[2],
		Path:  fields[3],
	}, true
}

// sortVisits puts the most recent first.
//
// The sort is stable and the file is written newest first, so two jumps within
// the same second keep the order they happened in: the timestamps have
// second resolution, and `pj -` would otherwise toggle to the wrong one.
func sortVisits(visits []Visit) {
	sort.SliceStable(visits, func(i, j int) bool {
		return visits[i].At.After(visits[j].At)
	})
}

// RecordVisit remembers a jump, keeping one entry per project.
func RecordVisit(name, path string) error {
	if name == "" || name == PreviousName {
		return fmt.Errorf("cannot record a visit without a name")
	}

	visits := ReadHistory()
	now := time.Now().Truncate(time.Second)

	updated := make([]Visit, 0, len(visits)+1)
	count := 1
	for _, visit := range visits {
		if visit.Name == name {
			// One entry per project, so that `pj -` toggles between two
			// projects instead of walking back through the same one twice.
			count = visit.Count + 1
			continue
		}
		updated = append(updated, visit)
	}
	updated = append([]Visit{{Name: name, Path: path, Count: count, At: now}}, updated...)

	if len(updated) > HistoryLimit {
		updated = updated[:HistoryLimit]
	}
	return writeHistory(updated)
}

// PreviousVisit returns the project visited before the current one, which is
// what `pj -` asks for.
func PreviousVisit() (Visit, bool) {
	visits := ReadHistory()
	if len(visits) < 2 {
		return Visit{}, false
	}
	return visits[1], true
}

// ClearHistory forgets every visit.
func ClearHistory() error {
	if err := os.Remove(HistoryFile()); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("cannot clear the jump history: %w", err)
	}
	return nil
}

// writeHistory replaces the file, through a temporary one so that a jump
// interrupted halfway leaves the previous history rather than half of a new
// one.
func writeHistory(visits []Visit) error {
	path := HistoryFile()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("cannot create %s: %w", filepath.Dir(path), err)
	}

	temp, err := os.CreateTemp(filepath.Dir(path), ".history-*")
	if err != nil {
		return fmt.Errorf("cannot write the jump history: %w", err)
	}
	defer os.Remove(temp.Name())

	writer := bufio.NewWriter(temp)
	for _, visit := range visits {
		if _, err := fmt.Fprintf(writer, "%d\t%d\t%s\t%s\n",
			visit.At.Unix(), visit.Count, visit.Name, visit.Path); err != nil {
			temp.Close()
			return fmt.Errorf("cannot write the jump history: %w", err)
		}
	}
	if err := writer.Flush(); err != nil {
		temp.Close()
		return fmt.Errorf("cannot write the jump history: %w", err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("cannot write the jump history: %w", err)
	}

	if err := os.Rename(temp.Name(), path); err != nil {
		return fmt.Errorf("cannot write the jump history: %w", err)
	}
	return nil
}
