package folderutil

import (
	"reflect"
	"strings"
	"testing"

	"gitlab.com/dynamo-tools/projekt/pkg/lazypath"
)

// useHistoryFor points the jump history at a scratch file and records visits.
func useHistoryFor(t *testing.T, names ...string) {
	t.Helper()

	previous := lazypath.HistoryFileOverride
	lazypath.HistoryFileOverride = t.TempDir() + "/history.tsv"
	t.Cleanup(func() { lazypath.HistoryFileOverride = previous })

	for _, name := range names {
		if err := lazypath.RecordVisit(name, "/tmp/"+name); err != nil {
			t.Fatalf("RecordVisit(%s) error = %v", name, err)
		}
	}
}

func foldersNamed(names ...string) []ParsedFolder {
	folders := make([]ParsedFolder, 0, len(names))
	for _, name := range names {
		folders = append(folders, ParsedFolder{ShortName: name, Path: "/tmp/" + name})
	}
	return folders
}

func matchedNames(matches []Match) []string {
	names := make([]string, 0, len(matches))
	for _, match := range matches {
		names = append(names, match.Folder.ShortName)
	}
	return names
}

func TestMatchFolders_Kinds(t *testing.T) {
	useHistoryFor(t)
	folders := foldersNamed("backend-api", "BACKEND-API-DOCS", "api-gateway", "frontend", "notes")

	cases := map[string]string{
		"backend-api": "backend-api", // exact
		"backend":     "backend-api", // prefix
		"gateway":     "api-gateway", // substring
		"bak":         "backend-api", // subsequence: b, a, k in order
		"frnt":        "frontend",
	}
	for query, want := range cases {
		matches := MatchFolders(folders, query)
		if len(matches) == 0 {
			t.Errorf("MatchFolders(%q) found nothing", query)
			continue
		}
		if matches[0].Folder.ShortName != want {
			t.Errorf("MatchFolders(%q) = %v, want %q first", query, matchedNames(matches), want)
		}
	}

	if got := MatchFolders(folders, "zzz"); len(got) != 0 {
		t.Errorf("MatchFolders(\"zzz\") = %v, want nothing", matchedNames(got))
	}
}

func TestMatchFolders_ExactWins(t *testing.T) {
	useHistoryFor(t, "api-gateway", "api-gateway", "api-gateway")
	folders := foldersNamed("api", "api-gateway", "rapid")

	// "api" is a substring of two others and the one visited most is another,
	// but a whole name is never reinterpreted.
	matches := MatchFolders(folders, "api")
	if !matches[0].Exact() || matches[0].Folder.ShortName != "api" {
		t.Errorf("MatchFolders(\"api\") = %v, want the exact name first", matchedNames(matches))
	}
}

func TestMatchFolders_CloserKindBeatsFrecency(t *testing.T) {
	useHistoryFor(t, "rapid-prototype", "rapid-prototype", "rapid-prototype")
	folders := foldersNamed("api-gateway", "rapid-prototype")

	// "api" is a prefix of one and only a subsequence of the other, however
	// often the other has been visited.
	matches := MatchFolders(folders, "api")
	if matches[0].Folder.ShortName != "api-gateway" {
		t.Errorf("MatchFolders(\"api\") = %v, want the prefix match first", matchedNames(matches))
	}
}

func TestMatchFolders_FrecencyBreaksATie(t *testing.T) {
	folders := foldersNamed("service-one", "service-two")

	useHistoryFor(t, "service-two")
	matches := MatchFolders(folders, "service")
	if matches[0].Folder.ShortName != "service-two" {
		t.Errorf("MatchFolders(\"service\") = %v, want the one that was visited", matchedNames(matches))
	}

	// With nothing to go on the order is by name, so two runs agree.
	useHistoryFor(t)
	matches = MatchFolders(folders, "service")
	if !reflect.DeepEqual(matchedNames(matches), []string{"service-one", "service-two"}) {
		t.Errorf("MatchFolders(\"service\") = %v, want them in name order", matchedNames(matches))
	}
}

func TestMatchFolders_EmptyQueryIsEverything(t *testing.T) {
	useHistoryFor(t, "notes")
	folders := foldersNamed("alpha", "notes", "zulu")

	matches := MatchFolders(folders, "")
	if len(matches) != 3 {
		t.Fatalf("MatchFolders(\"\") = %v, want all of them", matchedNames(matches))
	}
	// Still ranked: the one you have been in comes first.
	if matches[0].Folder.ShortName != "notes" {
		t.Errorf("MatchFolders(\"\") = %v, want the visited one first", matchedNames(matches))
	}
}

func TestMatchFolders_CaseInsensitive(t *testing.T) {
	useHistoryFor(t)
	folders := foldersNamed("Backend-API")

	matches := MatchFolders(folders, "backend-api")
	if len(matches) != 1 || matches[0].Exact() {
		t.Errorf("MatchFolders() = %v, want a match that is not called exact", matchedNames(matches))
	}
}

func TestIsSubsequence(t *testing.T) {
	cases := map[string]bool{
		"bak":         true,
		"backend-api": true,
		"ba-i":        true,
		"kb":          false, // the letters are there, in the wrong order
		"xyz":         false,
		"":            true,
	}
	for query, want := range cases {
		if got := isSubsequence("backend-api", query); got != want {
			t.Errorf("isSubsequence(%q) = %v, want %v", query, got, want)
		}
	}

	// Compared rune by rune, so a name outside ASCII does not match nonsense.
	if !isSubsequence("dự-án-chính", "dán") {
		t.Error("isSubsequence() does not walk runes")
	}
	if isSubsequence("dự-án", "xyz") {
		t.Error("isSubsequence() matched letters that are not there")
	}
}

func TestResolveFolder_ExactOnly(t *testing.T) {
	useHistoryFor(t)
	lazypath.SetTestConfig(lazypath.Config{Folders: []lazypath.Folder{
		{Path: "/tmp/backend-api"},
	}})
	t.Cleanup(lazypath.ResetTestConfig)

	if _, err := ResolveFolder("bak", GetOptions{}); err != nil {
		t.Errorf("ResolveFolder(\"bak\") error = %v, want the loose match", err)
	}
	// A script would rather be told it was wrong than sent somewhere close.
	if _, err := ResolveFolder("bak", GetOptions{ExactOnly: true}); err == nil {
		t.Error("ResolveFolder() with ExactOnly error = nil, want a refusal")
	}
	if _, err := ResolveFolder("backend-api", GetOptions{ExactOnly: true}); err != nil {
		t.Errorf("ResolveFolder() with ExactOnly error = %v, want the whole name to work", err)
	}
}

func TestFindFolderByShortName_Loose(t *testing.T) {
	useHistoryFor(t)
	lazypath.SetTestConfig(lazypath.Config{Folders: []lazypath.Folder{
		{Path: "/tmp/backend-api"},
	}})
	t.Cleanup(lazypath.ResetTestConfig)

	var out strings.Builder
	if err := FindFolderByShortName(&out, "bak", GetOptions{NoRecord: true}); err != nil {
		t.Fatalf("FindFolderByShortName() error = %v", err)
	}
	if strings.TrimSpace(out.String()) != "/tmp/backend-api" {
		t.Errorf("got %q, want the loosely matched path", out.String())
	}
}
