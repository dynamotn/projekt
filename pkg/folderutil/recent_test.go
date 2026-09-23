package folderutil

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"gitlab.com/dynamo.foss/projekt/pkg/lazypath"
)

// useHistory points the jump history at a scratch file.
func useHistory(t *testing.T) {
	t.Helper()

	previous := lazypath.HistoryFileOverride
	lazypath.HistoryFileOverride = t.TempDir() + "/history.tsv"
	t.Cleanup(func() { lazypath.HistoryFileOverride = previous })
}

func TestListRecent(t *testing.T) {
	useHistory(t)

	for _, name := range []string{"alpha", "beta", "gamma"} {
		if err := lazypath.RecordVisit(name, "/tmp/"+name); err != nil {
			t.Fatalf("RecordVisit() error = %v", err)
		}
	}

	var out bytes.Buffer
	o := &RecentOptions{}
	o.Output = "tsv"
	o.NoHeaders = true
	if err := ListRecent(&out, o); err != nil {
		t.Fatalf("ListRecent() error = %v", err)
	}

	// Most recent first, not alphabetical: that is what makes it a history.
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 3 {
		t.Fatalf("got %d lines, want 3: %q", len(lines), out.String())
	}
	if !strings.HasPrefix(lines[0], "gamma\t") || !strings.HasPrefix(lines[2], "alpha\t") {
		t.Errorf("listing = %q, want gamma first and alpha last", out.String())
	}
}

func TestListRecent_TableKeepsTheOrder(t *testing.T) {
	useHistory(t)

	// Recorded oldest first, so recency and the alphabet disagree: that is
	// what makes this test able to tell them apart.
	for _, name := range []string{"alpha", "zulu"} {
		if err := lazypath.RecordVisit(name, "/tmp/"+name); err != nil {
			t.Fatalf("RecordVisit() error = %v", err)
		}
	}

	var out bytes.Buffer
	o := &RecentOptions{}
	o.NoColor = true
	if err := ListRecent(&out, o); err != nil {
		t.Fatalf("ListRecent() error = %v", err)
	}

	// The table renderer sorts by the first column unless it is told the rows
	// already have an order. Alphabetically "alpha" wins; by recency it does
	// not, and recency is what was asked for.
	if strings.Index(out.String(), "zulu") > strings.Index(out.String(), "alpha") {
		t.Errorf("table = %q, want the most recent first", out.String())
	}
}

func TestListRecent_Limit(t *testing.T) {
	useHistory(t)

	for _, name := range []string{"alpha", "beta", "gamma"} {
		if err := lazypath.RecordVisit(name, "/tmp/"+name); err != nil {
			t.Fatalf("RecordVisit() error = %v", err)
		}
	}

	var out bytes.Buffer
	o := &RecentOptions{Limit: 2, NamesOnly: true}
	o.Output = "tsv"
	o.NoHeaders = true
	if err := ListRecent(&out, o); err != nil {
		t.Fatalf("ListRecent() error = %v", err)
	}
	if got := strings.TrimSpace(out.String()); got != "gamma\nbeta" {
		t.Errorf("listing = %q, want the two most recent", got)
	}
}

func TestListRecent_EmptyJSONIsAnArray(t *testing.T) {
	useHistory(t)

	var out bytes.Buffer
	o := &RecentOptions{}
	o.Output = "json"
	if err := ListRecent(&out, o); err != nil {
		t.Fatalf("ListRecent() error = %v", err)
	}
	if strings.TrimSpace(out.String()) != "[]" {
		t.Errorf("listing = %q, want []", out.String())
	}
}

func TestFindFolderByShortName_RecordsAndGoesBack(t *testing.T) {
	useHistory(t)

	root := t.TempDir()
	lazypath.SetTestConfig(lazypath.Config{Folders: []lazypath.Folder{
		{Path: root + "/alpha"},
		{Path: root + "/beta"},
	}})
	t.Cleanup(lazypath.ResetTestConfig)

	var out bytes.Buffer
	for _, name := range []string{"alpha", "beta"} {
		out.Reset()
		if err := FindFolderByShortName(&out, name, GetOptions{}); err != nil {
			t.Fatalf("FindFolderByShortName(%s) error = %v", name, err)
		}
	}

	// "-" is the project before this one.
	out.Reset()
	if err := FindFolderByShortName(&out, lazypath.PreviousName, GetOptions{}); err != nil {
		t.Fatalf("FindFolderByShortName(-) error = %v", err)
	}
	if strings.TrimSpace(out.String()) != root+"/alpha" {
		t.Errorf("got %q, want alpha", out.String())
	}

	// And going back is itself a jump, so it toggles.
	out.Reset()
	if err := FindFolderByShortName(&out, lazypath.PreviousName, GetOptions{}); err != nil {
		t.Fatalf("FindFolderByShortName(-) error = %v", err)
	}
	if strings.TrimSpace(out.String()) != root+"/beta" {
		t.Errorf("got %q, want beta", out.String())
	}
}

func TestFindFolderByShortName_NoRecord(t *testing.T) {
	useHistory(t)

	root := t.TempDir()
	lazypath.SetTestConfig(lazypath.Config{Folders: []lazypath.Folder{{Path: root + "/alpha"}}})
	t.Cleanup(lazypath.ResetTestConfig)

	var out bytes.Buffer
	if err := FindFolderByShortName(&out, "alpha", GetOptions{NoRecord: true}); err != nil {
		t.Fatalf("FindFolderByShortName() error = %v", err)
	}
	if visits := lazypath.ReadHistory(); len(visits) != 0 {
		t.Errorf("history = %v, want a lookup that is not a jump to leave it alone", visits)
	}
}

func TestFindFolderByShortName_NoPrevious(t *testing.T) {
	useHistory(t)
	lazypath.SetTestConfig(lazypath.Config{})
	t.Cleanup(lazypath.ResetTestConfig)

	var out bytes.Buffer
	if err := FindFolderByShortName(&out, lazypath.PreviousName, GetOptions{}); err == nil {
		t.Error("FindFolderByShortName(-) with no history error = nil, want an error")
	}
}

func TestAgo(t *testing.T) {
	now := time.Now()
	cases := map[time.Duration]string{
		10 * time.Second:     "just now",
		time.Minute:          "1 minute ago",
		90 * time.Minute:     "1 hour ago",
		50 * time.Hour:       "2 days ago",
		400 * 24 * time.Hour: "1 year ago",
	}
	for elapsed, want := range cases {
		if got := Ago(now.Add(-elapsed)); got != want {
			t.Errorf("Ago(%v ago) = %q, want %q", elapsed, got, want)
		}
	}
}
