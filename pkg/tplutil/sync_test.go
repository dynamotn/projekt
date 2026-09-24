package tplutil

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// originRepo builds a small repository of templates to clone from.
func originRepo(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("no git to clone with")
	}

	dir := t.TempDir()
	writeTemplate(t, filepath.Join(dir, "adr.tmpl"), "# {{ .Values.title }}\n")
	writeTemplate(t, filepath.Join(dir, "note.tmpl"), "note\n")
	git(t, dir, "init", "-q", "-b", "main", ".")
	git(t, dir, "add", "-A")
	git(t, dir, "-c", "user.email=t@example.com", "-c", "user.name=t", "commit", "-qm", "templates")
	return dir
}

func git(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
}

// useStoreAt points the store at a folder that does not exist yet, which is
// what `t init` expects to find.
func useStoreAt(t *testing.T, dir string) {
	t.Helper()
	previous := TemplateDir
	TemplateDir = dir
	t.Setenv("PROJEKT_TEMPLATE_DIR", "")
	t.Cleanup(func() { TemplateDir = previous })
}

func TestInitStore_ClonesAndThenSyncs(t *testing.T) {
	origin := originRepo(t)
	store := filepath.Join(t.TempDir(), "templates")
	useStoreAt(t, store)

	var out bytes.Buffer
	if err := InitStore(&out, origin, ""); err != nil {
		t.Fatalf("InitStore() error = %v", err)
	}
	if !strings.Contains(out.String(), "2 templates") {
		t.Errorf("InitStore() said %q, want the templates it found", out.String())
	}

	status, err := Status()
	if err != nil {
		t.Fatalf("Status() error = %v", err)
	}
	if !status.IsRepo || status.Remote != origin {
		t.Errorf("Status() = %+v, want a repository pointing at the origin", status)
	}

	// The store follows its remote.
	writeTemplate(t, filepath.Join(origin, "third.tmpl"), "third\n")
	git(t, origin, "add", "-A")
	git(t, origin, "-c", "user.email=t@example.com", "-c", "user.name=t", "commit", "-qm", "third")

	out.Reset()
	if err := SyncStore(&out, false); err != nil {
		t.Fatalf("SyncStore() error = %v", err)
	}
	templates, err := List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(templates) != 3 {
		t.Errorf("List() = %v, want the third template pulled in", templates)
	}
}

func TestInitStore_RefusesToCloneOverTemplates(t *testing.T) {
	origin := originRepo(t)
	store := useStore(t)
	writeTemplate(t, filepath.Join(store, "mine.tmpl"), "mine\n")

	err := InitStore(&bytes.Buffer{}, origin, "")
	if err == nil || !strings.Contains(err.Error(), "already holds templates") {
		t.Errorf("InitStore() error = %v, want it to refuse to clone over a store", err)
	}
	if _, statErr := os.Stat(filepath.Join(store, "mine.tmpl")); statErr != nil {
		t.Errorf("InitStore() removed a template it refused to replace: %v", statErr)
	}
}

func TestInitStore_RefusesASecondClone(t *testing.T) {
	origin := originRepo(t)
	useStoreAt(t, filepath.Join(t.TempDir(), "templates"))
	if err := InitStore(&bytes.Buffer{}, origin, ""); err != nil {
		t.Fatalf("InitStore() error = %v", err)
	}

	err := InitStore(&bytes.Buffer{}, origin, "")
	if err == nil || !strings.Contains(err.Error(), "t sync") {
		t.Errorf("InitStore() error = %v, want it to point at sync instead", err)
	}
}

func TestSyncStore_RefusesAStoreThatOnlySitsInsideARepository(t *testing.T) {
	// The store is a plain folder inside a checkout — pulling it would pull
	// the checkout, which is not what updating a template store means.
	repo := originRepo(t)
	store := filepath.Join(repo, "nested", "templates")
	writeTemplate(t, filepath.Join(store, "mine.tmpl"), "mine\n")
	useStoreAt(t, store)

	err := SyncStore(&bytes.Buffer{}, false)
	if err == nil || !strings.Contains(err.Error(), "not a git repository") {
		t.Errorf("SyncStore() error = %v, want it to refuse a folder inside another repository", err)
	}
}

func TestSyncStore_SaysWhatIsMissing(t *testing.T) {
	useStoreAt(t, filepath.Join(t.TempDir(), "nowhere"))
	err := SyncStore(&bytes.Buffer{}, false)
	if err == nil || !strings.Contains(err.Error(), "t init") {
		t.Errorf("SyncStore() error = %v, want it to point at init", err)
	}
}

func TestSyncStore_DryRunTouchesNothing(t *testing.T) {
	origin := originRepo(t)
	useStoreAt(t, filepath.Join(t.TempDir(), "templates"))
	if err := InitStore(&bytes.Buffer{}, origin, ""); err != nil {
		t.Fatalf("InitStore() error = %v", err)
	}

	writeTemplate(t, filepath.Join(origin, "third.tmpl"), "third\n")
	git(t, origin, "add", "-A")
	git(t, origin, "-c", "user.email=t@example.com", "-c", "user.name=t", "commit", "-qm", "third")

	var out bytes.Buffer
	if err := SyncStore(&out, true); err != nil {
		t.Fatalf("SyncStore() error = %v", err)
	}
	if !strings.Contains(out.String(), "[DRY RUN]") {
		t.Errorf("SyncStore() said %q, want it to describe the pull", out.String())
	}
	templates, err := List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(templates) != 2 {
		t.Errorf("a dry run pulled: List() = %v", templates)
	}
}
