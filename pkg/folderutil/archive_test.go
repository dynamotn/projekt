package folderutil

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"go.yaml.in/yaml/v3"

	"gitlab.com/dynamo-tools/projekt/pkg/lazypath"
)

// archivable sets up a project, an archive folder and a writable config.
func archivable(t *testing.T) (project, archive string) {
	t.Helper()

	root := t.TempDir()
	project = filepath.Join(root, "old-service")
	archive = filepath.Join(root, "archive")
	initRepo(t, project)

	configFile := filepath.Join(root, "config.yaml")
	previousCfg := lazypath.CfgFile
	lazypath.CfgFile = configFile
	lazypath.ResetTestConfig()
	lazypath.InitConfig()

	previousArchive := ArchiveDir
	ArchiveDir = archive
	t.Setenv("PROJEKT_ARCHIVE_DIR", "")
	t.Cleanup(func() {
		lazypath.CfgFile = previousCfg
		lazypath.ResetTestConfig()
		ArchiveDir = previousArchive
	})

	lazypath.SetTestConfig(lazypath.Config{Folders: []lazypath.Folder{{Path: project}}})
	return project, archive
}

// pushable gives the repository an upstream with everything on it, so that it
// counts as finished.
func pushable(t *testing.T, project string) {
	t.Helper()

	origin := project + ".git"
	// The branch is named, as initRepo names it: a machine without
	// init.defaultBranch would point the bare HEAD at master, and a clone of it
	// would check out a branch that does not exist.
	if err := runGit(filepath.Dir(project), "init", "--bare", "--initial-branch=main", origin); err != nil {
		t.Fatalf("init --bare: %v", err)
	}
	mustGit(t, project, "remote", "add", "origin", origin)
	mustGit(t, project, "push", "--quiet", "--set-upstream", "origin", "main")
}

func TestArchiveFolder(t *testing.T) {
	project, archive := archivable(t)
	pushable(t, project)

	var out bytes.Buffer
	if err := ArchiveFolder(&out, "old-service", ArchiveOptions{}); err != nil {
		t.Fatalf("ArchiveFolder() error = %v", err)
	}

	target := filepath.Join(archive, "old-service")
	if _, err := os.Stat(filepath.Join(target, "README")); err != nil {
		t.Errorf("the project is not in the archive: %v", err)
	}
	if _, err := os.Stat(project); !os.IsNotExist(err) {
		t.Error("the project is still where it was")
	}
	if folders := lazypath.GetConfig().Folders; len(folders) != 0 {
		t.Errorf("config = %#v, want the entry dropped", folders)
	}
	if !strings.Contains(out.String(), target) {
		t.Errorf("output = %q, want it to say where the project went", out.String())
	}
}

func TestArchiveFolder_RefusesUnfinishedWork(t *testing.T) {
	project, _ := archivable(t)
	pushable(t, project)

	// Something not committed.
	if err := os.WriteFile(filepath.Join(project, "wip"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var out bytes.Buffer
	err := ArchiveFolder(&out, "old-service", ArchiveOptions{})
	if err == nil {
		t.Fatal("ArchiveFolder() error = nil, want a refusal")
	}
	if !strings.Contains(err.Error(), "not committed") || !strings.Contains(err.Error(), "--force") {
		t.Errorf("error = %v, want the reason and the way out", err)
	}
	if _, statErr := os.Stat(project); statErr != nil {
		t.Error("the refusal moved the project anyway")
	}

	// --force says you mean it.
	if err := ArchiveFolder(&out, "old-service", ArchiveOptions{Force: true}); err != nil {
		t.Fatalf("ArchiveFolder() with Force error = %v", err)
	}
}

func TestArchiveFolder_RefusesUnpushedCommits(t *testing.T) {
	project, _ := archivable(t)
	pushable(t, project)

	if err := os.WriteFile(filepath.Join(project, "more"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	mustGit(t, project, "add", "more")
	mustGit(t, project, "commit", "-m", "not pushed")

	var out bytes.Buffer
	err := ArchiveFolder(&out, "old-service", ArchiveOptions{})
	if err == nil || !strings.Contains(err.Error(), "not pushed") {
		t.Errorf("ArchiveFolder() error = %v, want it to refuse unpushed commits", err)
	}
}

func TestArchiveFolder_RefusesARepositoryWithNoUpstream(t *testing.T) {
	archivable(t)

	// Never pushed anywhere is the one worth stopping for hardest.
	var out bytes.Buffer
	err := ArchiveFolder(&out, "old-service", ArchiveOptions{})
	if err == nil || !strings.Contains(err.Error(), "upstream") {
		t.Errorf("ArchiveFolder() error = %v, want it to refuse a repository with no upstream", err)
	}
}

func TestArchiveFolder_RefusesAStash(t *testing.T) {
	project, _ := archivable(t)
	pushable(t, project)

	if err := os.WriteFile(filepath.Join(project, "wip"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	mustGit(t, project, "add", "wip")
	mustGit(t, project, "stash", "push", "-m", "later")

	var out bytes.Buffer
	err := ArchiveFolder(&out, "old-service", ArchiveOptions{})
	if err == nil || !strings.Contains(err.Error(), "stash") {
		t.Errorf("ArchiveFolder() error = %v, want it to refuse a stash", err)
	}
}

func TestArchiveFolder_PlainFolderIsNotGuessedAbout(t *testing.T) {
	root := t.TempDir()
	project := filepath.Join(root, "notes")
	if err := os.MkdirAll(project, 0o755); err != nil {
		t.Fatal(err)
	}

	configFile := filepath.Join(root, "config.yaml")
	previousCfg := lazypath.CfgFile
	lazypath.CfgFile = configFile
	lazypath.ResetTestConfig()
	lazypath.InitConfig()
	previousArchive := ArchiveDir
	ArchiveDir = filepath.Join(root, "archive")
	t.Setenv("PROJEKT_ARCHIVE_DIR", "")
	t.Cleanup(func() {
		lazypath.CfgFile = previousCfg
		lazypath.ResetTestConfig()
		ArchiveDir = previousArchive
	})
	lazypath.SetTestConfig(lazypath.Config{Folders: []lazypath.Folder{{Path: project}}})

	// Not a repository: there is no way to tell what is finished, so it is not
	// this command's place to guess.
	var out bytes.Buffer
	if err := ArchiveFolder(&out, "notes", ArchiveOptions{}); err != nil {
		t.Errorf("ArchiveFolder() of a plain folder error = %v", err)
	}
}

func TestArchiveFolder_DryRun(t *testing.T) {
	project, archive := archivable(t)
	pushable(t, project)

	var out bytes.Buffer
	if err := ArchiveFolder(&out, "old-service", ArchiveOptions{DryRun: true}); err != nil {
		t.Fatalf("ArchiveFolder() error = %v", err)
	}
	if !strings.Contains(out.String(), "DRY RUN") {
		t.Errorf("output = %q, want the plan", out.String())
	}
	if _, err := os.Stat(project); err != nil {
		t.Error("the dry run moved the project")
	}
	if _, err := os.Stat(archive); !os.IsNotExist(err) {
		t.Error("the dry run created the archive folder")
	}
	if len(lazypath.GetConfig().Folders) != 1 {
		t.Error("the dry run changed the configuration")
	}
}

func TestArchiveFolder_Errors(t *testing.T) {
	project, archive := archivable(t)
	pushable(t, project)

	var out bytes.Buffer
	if err := ArchiveFolder(&out, "nope", ArchiveOptions{}); err == nil {
		t.Error("ArchiveFolder() of an unknown project error = nil, want an error")
	}
	// A worktree is put away by its own command.
	if err := ArchiveFolder(&out, "old-service@wt", ArchiveOptions{}); err == nil {
		t.Error("ArchiveFolder() of a worktree name error = nil, want an error")
	}

	// Something already archived under that name is not silently replaced.
	if err := os.MkdirAll(filepath.Join(archive, "old-service"), 0o755); err != nil {
		t.Fatal(err)
	}
	err := ArchiveFolder(&out, "old-service", ArchiveOptions{})
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Errorf("ArchiveFolder() error = %v, want it to refuse to overwrite", err)
	}
}

func TestArchiveRoot(t *testing.T) {
	previous := ArchiveDir
	ArchiveDir = ""
	t.Setenv("PROJEKT_ARCHIVE_DIR", "")
	t.Cleanup(func() { ArchiveDir = previous })

	if got, _ := archiveRoot(ArchiveOptions{}); got != DefaultArchiveDir() {
		t.Errorf("archiveRoot() = %v, want the default", got)
	}

	t.Setenv("PROJEKT_ARCHIVE_DIR", "/tmp/from-env")
	if got, _ := archiveRoot(ArchiveOptions{}); got != "/tmp/from-env" {
		t.Errorf("archiveRoot() = %v, want the environment", got)
	}

	ArchiveDir = "/tmp/from-flag"
	if got, _ := archiveRoot(ArchiveOptions{}); got != "/tmp/from-flag" {
		t.Errorf("archiveRoot() = %v, want the flag to win", got)
	}

	if got, _ := archiveRoot(ArchiveOptions{To: "/tmp/from-to"}); got != "/tmp/from-to" {
		t.Errorf("archiveRoot() = %v, want --to to win", got)
	}
}

func TestArchiveFolder_WritesARecord(t *testing.T) {
	project, archive := archivable(t)
	pushable(t, project)

	if err := ArchiveFolder(&bytes.Buffer{}, "old-service", ArchiveOptions{}); err != nil {
		t.Fatalf("ArchiveFolder() error = %v", err)
	}

	data, err := os.ReadFile(filepath.Join(archive, "old-service", ".projekt", "archived.yaml"))
	if err != nil {
		t.Fatalf("no record: %v", err)
	}
	var record ArchiveRecord
	if err := yaml.Unmarshal(data, &record); err != nil {
		t.Fatalf("record does not decode: %v\n%s", err, data)
	}
	if record.Original != project {
		t.Errorf("Original = %q, want %q", record.Original, project)
	}
	if record.Entry == nil || record.Entry.Path != project {
		t.Errorf("Entry = %#v, want the dropped entry", record.Entry)
	}
	if time.Since(record.ArchivedAt) > time.Minute {
		t.Errorf("ArchivedAt = %v, want now", record.ArchivedAt)
	}
}

func TestArchiveFolder_RefusesALinkedProjektFolderBeforeMoving(t *testing.T) {
	project, _ := archivable(t)
	pushable(t, project)
	if err := os.Symlink(t.TempDir(), filepath.Join(project, ".projekt")); err != nil {
		t.Fatal(err)
	}

	err := ArchiveFolder(&bytes.Buffer{}, "old-service", ArchiveOptions{Force: true})
	if err == nil || !strings.Contains(err.Error(), "symbolic link") {
		t.Fatalf("error = %v, want a refusal naming the link", err)
	}
	if _, statErr := os.Stat(project); statErr != nil {
		t.Errorf("the project was moved: %v", statErr)
	}
}

func TestArchiveFolder_RefusesWorktrees(t *testing.T) {
	for _, configured := range []bool{true, false} {
		project, archive := archivable(t)
		tree := filepath.Join(filepath.Dir(project), "tree")
		mustGit(t, project, "worktree", "add", "-b", "side", tree)
		if configured {
			lazypath.SetTestConfig(lazypath.Config{
				Folders:   []lazypath.Folder{{Path: project}},
				Worktrees: []lazypath.Worktree{{Project: "old-service", Name: "side", Branch: "side", Path: tree}},
			})
		}

		err := ArchiveFolder(&bytes.Buffer{}, "old-service", ArchiveOptions{Force: true})
		if err == nil || !strings.Contains(err.Error(), "remove its working trees first") {
			t.Errorf("configured=%v: error = %v, want a refusal", configured, err)
		}
		if _, statErr := os.Stat(project); statErr != nil {
			t.Errorf("configured=%v: the project was moved", configured)
		}
		if _, statErr := os.Stat(archive); !os.IsNotExist(statErr) {
			t.Errorf("configured=%v: the archive was created", configured)
		}
	}
}

func TestArchiveFolder_RefusesAnIncludedProject(t *testing.T) {
	project, _ := archivable(t)
	lazypath.SetTestConfig(lazypath.Config{Folders: []lazypath.Folder{{Path: project, Source: "/etc/team.yaml"}}})

	err := ArchiveFolder(&bytes.Buffer{}, "old-service", ArchiveOptions{Force: true})
	if err == nil || !strings.Contains(err.Error(), "declared in /etc/team.yaml; archive it there") {
		t.Fatalf("error = %v, want a refusal naming the file", err)
	}
	if _, statErr := os.Stat(project); statErr != nil {
		t.Errorf("the project was moved: %v", statErr)
	}
}

func TestListArchive(t *testing.T) {
	archive := t.TempDir()
	at := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	data, err := yaml.Marshal(ArchiveRecord{Original: "/old/b", ArchivedAt: at})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(archive, "b", ".projekt"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(archive, "b", ".projekt", "archived.yaml"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(filepath.Join(archive, "a"), 0o755); err != nil {
		t.Fatal(err)
	}

	got, err := ListArchive(ArchiveOptions{To: archive})
	if err != nil {
		t.Fatalf("ListArchive() error = %v", err)
	}
	if len(got) != 2 || got[0].Name != "a" || got[1].Name != "b" {
		t.Fatalf("got %#v, want a then b", got)
	}
	if got[0].Original != "" || !got[0].At.IsZero() {
		t.Errorf("a = %#v, want no record", got[0])
	}
	if got[1].Original != "/old/b" || !got[1].At.Equal(at) {
		t.Errorf("b = %#v, want its record", got[1])
	}

	if none, err := ListArchive(ArchiveOptions{To: filepath.Join(archive, "missing")}); err != nil || len(none) != 0 {
		t.Errorf("missing archive = %v, %v, want an empty list", none, err)
	}
}
