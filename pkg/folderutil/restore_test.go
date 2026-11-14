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
	if err := RestoreFolder(&out, "old-service", RestoreOptions{}); err != nil {
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
}

func TestRestoreFolder_RefusesALyingRecord(t *testing.T) {
	_, archive := archivable(t)
	for name, original := range map[string]string{
		"empty":    "",
		"relative": "relative/path",
		"inside":   filepath.Join(archive, "elsewhere"),
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
