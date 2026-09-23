package lazypath

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// useHistory points the jump history at a scratch file.
func useHistory(t *testing.T) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "history.tsv")
	previous := HistoryFileOverride
	HistoryFileOverride = path
	t.Cleanup(func() { HistoryFileOverride = previous })
	return path
}

func TestStateHome(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", "/tmp/state")
	if got := StateHome(); got != "/tmp/state" {
		t.Errorf("StateHome() = %v, want /tmp/state", got)
	}

	// The spec's default, for a machine that does not set it.
	t.Setenv("XDG_STATE_HOME", "")
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("no home directory")
	}
	if got, want := StateHome(), filepath.Join(home, ".local", "state"); got != want {
		t.Errorf("StateHome() = %v, want %v", got, want)
	}
}

func TestReadHistory_Missing(t *testing.T) {
	useHistory(t)

	// Nothing jumped to yet is not an error; it is an empty list.
	if visits := ReadHistory(); len(visits) != 0 {
		t.Errorf("ReadHistory() = %v, want nothing", visits)
	}
	if _, ok := PreviousVisit(); ok {
		t.Error("PreviousVisit() found one in an empty history")
	}
}

func TestRecordVisit(t *testing.T) {
	path := useHistory(t)

	if err := RecordVisit("alpha", "/tmp/alpha"); err != nil {
		t.Fatalf("RecordVisit() error = %v", err)
	}
	if err := RecordVisit("beta", "/tmp/beta"); err != nil {
		t.Fatalf("RecordVisit() error = %v", err)
	}

	visits := ReadHistory()
	if len(visits) != 2 {
		t.Fatalf("ReadHistory() = %v, want two", visits)
	}
	// Most recent first.
	if visits[0].Name != "beta" || visits[1].Name != "alpha" {
		t.Errorf("ReadHistory() = %v, want beta then alpha", visits)
	}
	if visits[0].Path != "/tmp/beta" || visits[0].Count != 1 {
		t.Errorf("first = %#v", visits[0])
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if !strings.Contains(string(data), "\talpha\t/tmp/alpha") {
		t.Errorf("history file = %q", data)
	}
}

func TestRecordVisit_OneEntryPerProject(t *testing.T) {
	useHistory(t)

	for i := 0; i < 3; i++ {
		if err := RecordVisit("alpha", "/tmp/alpha"); err != nil {
			t.Fatalf("RecordVisit() error = %v", err)
		}
	}

	visits := ReadHistory()
	if len(visits) != 1 {
		t.Fatalf("ReadHistory() = %v, want one entry per project", visits)
	}
	if visits[0].Count != 3 {
		t.Errorf("Count = %d, want 3", visits[0].Count)
	}
}

func TestPreviousVisit_Toggles(t *testing.T) {
	useHistory(t)

	mustRecord(t, "alpha", "/tmp/alpha")
	mustRecord(t, "beta", "/tmp/beta")

	// In beta, `pj -` goes back to alpha.
	previous, ok := PreviousVisit()
	if !ok || previous.Name != "alpha" {
		t.Fatalf("PreviousVisit() = %#v, %v, want alpha", previous, ok)
	}

	// Jumping there makes beta the one to go back to, so it toggles.
	mustRecord(t, "alpha", "/tmp/alpha")
	previous, ok = PreviousVisit()
	if !ok || previous.Name != "beta" {
		t.Errorf("PreviousVisit() = %#v, %v, want beta", previous, ok)
	}
}

func TestRecordVisit_Bounded(t *testing.T) {
	useHistory(t)

	for i := 0; i < HistoryLimit+20; i++ {
		mustRecord(t, "project-"+string(rune('a'+i%26))+strings.Repeat("x", i/26), "/tmp/x")
	}

	if got := len(ReadHistory()); got > HistoryLimit {
		t.Errorf("ReadHistory() = %d entries, want at most %d", got, HistoryLimit)
	}
}

func TestRecordVisit_Errors(t *testing.T) {
	useHistory(t)

	if err := RecordVisit("", "/tmp/x"); err == nil {
		t.Error("RecordVisit() with no name error = nil, want an error")
	}
	if err := RecordVisit(PreviousName, "/tmp/x"); err == nil {
		t.Error("RecordVisit(\"-\") error = nil, want an error: it is not a project")
	}
}

func TestReadHistory_SkipsBrokenLines(t *testing.T) {
	path := useHistory(t)

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	content := strings.Join([]string{
		"not a history line",
		"abc\t1\tname\t/path",
		"123\tnotanumber\tname\t/path",
		"123\t1\t\t/path",
		"1700000000\t2\tgood\t/tmp/good",
		"",
	}, "\n")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	visits := ReadHistory()
	if len(visits) != 1 || visits[0].Name != "good" {
		t.Errorf("ReadHistory() = %v, want only the readable line", visits)
	}
	if visits[0].At.Unix() != 1700000000 || visits[0].Count != 2 {
		t.Errorf("visit = %#v", visits[0])
	}
}

func TestClearHistory(t *testing.T) {
	useHistory(t)
	mustRecord(t, "alpha", "/tmp/alpha")

	if err := ClearHistory(); err != nil {
		t.Fatalf("ClearHistory() error = %v", err)
	}
	if visits := ReadHistory(); len(visits) != 0 {
		t.Errorf("ReadHistory() = %v, want nothing", visits)
	}
	// Clearing an already empty history is not an error.
	if err := ClearHistory(); err != nil {
		t.Errorf("ClearHistory() twice error = %v", err)
	}
}

func TestSortVisits_KeepsTheOrderWithinASecond(t *testing.T) {
	at := time.Unix(1700000000, 0)
	// Written newest first, so the newest stays first: the timestamps only
	// have second resolution, and `pj -` depends on the order being right.
	visits := []Visit{{Name: "newest", At: at}, {Name: "older", At: at}}

	sortVisits(visits)
	if visits[0].Name != "newest" {
		t.Errorf("sortVisits() = %v, want the file order kept", visits)
	}
}

func mustRecord(t *testing.T, name, path string) {
	t.Helper()

	if err := RecordVisit(name, path); err != nil {
		t.Fatalf("RecordVisit(%s) error = %v", name, err)
	}
}
