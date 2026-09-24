package tplutil

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"gitlab.com/dynamo.foss/projekt/pkg/cli"
	"gitlab.com/dynamo.foss/projekt/pkg/folderutil"
)

// StoreStatus is what the template store is, as far as git is concerned.
type StoreStatus struct {
	// Dir is the store folder, whether or not anything is in it.
	Dir string
	// Exists reports whether the folder is there at all.
	Exists bool
	// IsRepo reports whether it is a git repository, which is what makes it
	// syncable.
	IsRepo bool
	// Remote is where `t sync` pulls from, when there is one.
	Remote string
	// Templates is how many templates the store holds.
	Templates int
}

// Status describes the template store.
func Status() (StoreStatus, error) {
	dir, err := Dir()
	if err != nil {
		return StoreStatus{}, err
	}
	status := StoreStatus{Dir: dir}

	info, err := os.Stat(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return status, nil
		}
		return StoreStatus{}, fmt.Errorf("cannot access %s: %w", dir, err)
	}
	if !info.IsDir() {
		return StoreStatus{}, fmt.Errorf("%s is a file, the template store must be a folder", dir)
	}
	status.Exists = true

	templates, err := List()
	if err != nil {
		return StoreStatus{}, err
	}
	status.Templates = len(templates)

	if !folderutil.IsRepoRoot(dir) {
		return status, nil
	}
	status.IsRepo = true
	status.Remote = folderutil.RemoteURL(dir, "origin")
	return status, nil
}

// InitStore clones a repository of templates into the store.
//
// Carrying a template store between machines was the one thing you had to do
// by hand; a store is a folder of files like any other, so it can simply be a
// repository.
func InitStore(out io.Writer, repo, ref string) error {
	status, err := Status()
	if err != nil {
		return err
	}

	switch {
	case status.IsRepo:
		return fmt.Errorf("%s is already a repository (%s); use `t sync` to update it", status.Dir, describeRemote(status.Remote))
	case status.Exists && !empty(status.Dir):
		// Cloning over it would either fail or lose what is there; neither is
		// something to do without being asked.
		return fmt.Errorf("%s already holds templates; move it aside first, or point --template-dir somewhere else", status.Dir)
	}

	ref2, err := folderutil.ParseRepoRef(repo)
	if err != nil {
		return err
	}
	primary, fallback, err := ref2.URLs()
	if err != nil {
		return err
	}

	// The store is worked in and pushed from, so it gets its history.
	if err := os.MkdirAll(filepath.Dir(status.Dir), 0o755); err != nil {
		return fmt.Errorf("cannot create %s: %w", filepath.Dir(status.Dir), err)
	}
	if err := os.RemoveAll(status.Dir); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("cannot clear the empty %s: %w", status.Dir, err)
	}
	if err := cloneFullWithFallback(primary, fallback, ref, status.Dir); err != nil {
		return err
	}

	after, err := Status()
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(out, "Cloned %s into %s (%d templates)\n", primary, after.Dir, after.Templates)
	return err
}

// cloneFullWithFallback clones with history, from the second URL when the
// first will not have it — the same two-URL dance `folder sync` does.
func cloneFullWithFallback(primary, fallback, ref, target string) error {
	err := folderutil.CloneFull(primary, ref, target)
	if err == nil || strings.TrimSpace(fallback) == "" {
		return err
	}
	cli.Debug("Cloning %s failed, trying %s", primary, fallback)
	if fallbackErr := folderutil.CloneFull(fallback, ref, target); fallbackErr != nil {
		return fmt.Errorf("%w; and %v", err, fallbackErr)
	}
	return nil
}

// SyncStore brings the store up to date with its remote.
//
// It pulls with --ff-only: a store with local edits is something to sort out
// by hand, not something for a sync to guess at.
func SyncStore(out io.Writer, dryRun bool) error {
	status, err := Status()
	if err != nil {
		return err
	}
	switch {
	case !status.Exists:
		return fmt.Errorf("%s does not exist; `t init <repo>` clones a store into it", status.Dir)
	case !status.IsRepo:
		return fmt.Errorf("%s is not a git repository; `t init <repo>` clones one, or make it one yourself", status.Dir)
	case status.Remote == "":
		return fmt.Errorf("%s has no origin to pull from", status.Dir)
	}

	if dryRun {
		_, err := fmt.Fprintf(out, "[DRY RUN] Would pull %s in %s\n", status.Remote, status.Dir)
		return err
	}

	if err := folderutil.Pull(status.Dir); err != nil {
		return err
	}

	after, err := Status()
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(out, "Updated %s from %s (%d templates)\n", after.Dir, after.Remote, after.Templates)
	return err
}

// describeRemote names a remote for a message, for a repository that has none.
func describeRemote(remote string) string {
	if remote == "" {
		return "no remote"
	}
	return remote
}

// empty reports whether a folder holds nothing.
func empty(dir string) bool {
	entries, err := os.ReadDir(dir)
	return err == nil && len(entries) == 0
}
