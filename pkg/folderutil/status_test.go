package folderutil

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gitlab.com/dynamo.foss/projekt/pkg/lazypath"
)

// statusFolders configures a clean repository, a dirty one, a plain folder and
// one that is not there.
func statusFolders(t *testing.T) (clean, dirty string) {
	t.Helper()

	root := t.TempDir()
	clean = filepath.Join(root, "clean")
	dirty = filepath.Join(root, "dirty")
	initRepo(t, clean)
	initRepo(t, dirty)

	if err := os.WriteFile(filepath.Join(dirty, "scratch"), []byte("wip\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	plain := filepath.Join(root, "plain")
	if err := os.MkdirAll(plain, 0o755); err != nil {
		t.Fatal(err)
	}

	lazypath.SetTestConfig(lazypath.Config{Folders: []lazypath.Folder{
		{Path: clean, Tags: []string{"work"}},
		{Path: dirty, Tags: []string{"work"}},
		{Path: plain},
		{Path: filepath.Join(root, "gone")},
	}})
	t.Cleanup(lazypath.ResetTestConfig)

	return clean, dirty
}

// statusOf reads the listing as JSON, which is the shape a test can assert on.
func statusOf(t *testing.T, o *StatusOptions) map[string]map[string]any {
	t.Helper()

	var out bytes.Buffer
	o.Output = "json"
	if err := StatusFolders(&out, o); err != nil {
		t.Fatalf("StatusFolders() error = %v", err)
	}

	var rows []map[string]any
	if err := json.Unmarshal(out.Bytes(), &rows); err != nil {
		t.Fatalf("json.Unmarshal(%q) error = %v", out.String(), err)
	}

	byName := make(map[string]map[string]any, len(rows))
	for _, row := range rows {
		byName[row["name"].(string)] = row
	}
	return byName
}

func TestStatusFolders(t *testing.T) {
	statusFolders(t)

	rows := statusOf(t, &StatusOptions{})
	if len(rows) != 4 {
		t.Fatalf("got %d rows, want 4: %v", len(rows), rows)
	}

	if rows["clean"]["state"] != statusOK || rows["clean"]["changed"].(float64) != 0 {
		t.Errorf("clean = %#v", rows["clean"])
	}
	if rows["clean"]["branch"] != "main" {
		t.Errorf("clean branch = %#v, want main", rows["clean"]["branch"])
	}
	if rows["clean"]["last"] == "" {
		t.Error("clean has no last commit time")
	}

	// An untracked file counts: git would mention it, so the listing does.
	if rows["dirty"]["changed"].(float64) != 1 {
		t.Errorf("dirty = %#v, want one changed file", rows["dirty"])
	}

	if rows["plain"]["state"] != statusNoRepo {
		t.Errorf("plain = %#v, want %q", rows["plain"], statusNoRepo)
	}
	if rows["gone"]["state"] != statusMissing {
		t.Errorf("gone = %#v, want %q", rows["gone"], statusMissing)
	}
}

func TestStatusFolders_DirtyOnly(t *testing.T) {
	statusFolders(t)

	rows := statusOf(t, &StatusOptions{DirtyOnly: true})
	if _, ok := rows["clean"]; ok {
		t.Errorf("--dirty kept a clean repository: %v", rows)
	}
	for _, name := range []string{"dirty", "plain", "gone"} {
		if _, ok := rows[name]; !ok {
			t.Errorf("--dirty dropped %q, which wants attention: %v", name, rows)
		}
	}
}

func TestStatusFolders_Tags(t *testing.T) {
	statusFolders(t)

	rows := statusOf(t, &StatusOptions{Tags: []string{"work"}})
	if len(rows) != 2 {
		t.Errorf("got %v, want only the two work folders", rows)
	}
}

func TestStatusFolders_AheadAndBehind(t *testing.T) {
	root := t.TempDir()
	origin := filepath.Join(root, "origin")
	clone := filepath.Join(root, "clone")
	initRepo(t, origin)
	mustGit(t, origin, "config", "receive.denyCurrentBranch", "ignore")

	if err := runGit(root, "clone", "--quiet", origin, clone); err != nil {
		t.Fatalf("clone: %v", err)
	}
	mustGit(t, clone, "config", "user.email", "test@example.com")
	mustGit(t, clone, "config", "user.name", "Test")
	if err := os.WriteFile(filepath.Join(clone, "new"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	mustGit(t, clone, "add", "new")
	mustGit(t, clone, "commit", "-m", "ahead by one")

	lazypath.SetTestConfig(lazypath.Config{Folders: []lazypath.Folder{{Path: clone}}})
	t.Cleanup(lazypath.ResetTestConfig)

	rows := statusOf(t, &StatusOptions{})
	if rows["clone"]["ahead"].(float64) != 1 {
		t.Errorf("clone = %#v, want one commit ahead", rows["clone"])
	}
	if rows["clone"]["behind"].(float64) != 0 {
		t.Errorf("clone = %#v, want nothing behind", rows["clone"])
	}
}

func TestStatusFolders_Stashes(t *testing.T) {
	_, dirty := statusFolders(t)

	// A stash is the easiest thing to forget, so it gets a column.
	mustGit(t, dirty, "add", "scratch")
	mustGit(t, dirty, "stash", "push", "-m", "wip")

	rows := statusOf(t, &StatusOptions{})
	if rows["dirty"]["stashes"].(float64) != 1 {
		t.Errorf("dirty = %#v, want one stash", rows["dirty"])
	}
}

func TestStatusFolders_Table(t *testing.T) {
	statusFolders(t)

	var out bytes.Buffer
	o := &StatusOptions{}
	o.NoColor = true
	if err := StatusFolders(&out, o); err != nil {
		t.Fatalf("StatusFolders() error = %v", err)
	}
	for _, want := range []string{"NAME", "BRANCH", "CHANGED", "STASH", "LAST COMMIT", "clean", "dirty"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("table = %q, want it to contain %q", out.String(), want)
		}
	}
}

func TestFolderStatus_Summary(t *testing.T) {
	cases := map[string]FolderStatus{
		"clean":                         {State: statusOK},
		"missing":                       {State: statusMissing},
		"2 changed, 1 ahead, 3 stashed": {State: statusOK, Changed: 2, Ahead: 1, Stashes: 3},
		"4 behind":                      {State: statusOK, Behind: 4},
	}
	for want, status := range cases {
		if got := status.Summary(); got != want {
			t.Errorf("Summary() = %q, want %q", got, want)
		}
	}

	if (FolderStatus{State: statusOK}).NeedsAttention() {
		t.Error("a clean folder wants attention")
	}
	if !(FolderStatus{State: statusOK, Stashes: 1}).NeedsAttention() {
		t.Error("a stash is worth attention")
	}
}

func TestParseAheadBehind(t *testing.T) {
	cases := map[string][2]int{
		"+1 -2":  {1, 2},
		"+0 -0":  {0, 0},
		"+12 -0": {12, 0},
		"":       {0, 0},
		"junk":   {0, 0},
	}
	for value, want := range cases {
		ahead, behind := parseAheadBehind(value)
		if ahead != want[0] || behind != want[1] {
			t.Errorf("parseAheadBehind(%q) = %d, %d, want %d, %d", value, ahead, behind, want[0], want[1])
		}
	}
}
