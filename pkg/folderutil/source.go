package folderutil

import (
	"fmt"
	"os"
	"strings"

	"gitlab.com/dynamo-tools/projekt/pkg/cli"
)

// GitURLs returns the URLs to try for a repository on a configured server:
// the preferred one and the one to fall back to.
func GitURLs(host, group, name string) (primary, fallback string, err error) {
	server := getGitServer(host)
	if server == nil {
		return "", "", fmt.Errorf("no git server named %q in the configuration", host)
	}
	primary, fallback = getGitURLs(server, group, name)
	return primary, fallback, nil
}

// IsGitURL reports whether a string is somewhere git can clone from outright,
// rather than the `server:group/name` shorthand.
func IsGitURL(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return false
	}
	if strings.Contains(value, "://") {
		return true
	}
	// A path on this machine: git clones from one, and a starting point kept
	// on a disk or a shared drive is a normal thing to have.
	if strings.HasPrefix(value, "/") || strings.HasPrefix(value, "~") ||
		strings.HasPrefix(value, "./") || strings.HasPrefix(value, "../") {
		return true
	}
	// scp syntax: git@host:group/repo.
	at := strings.Index(value, "@")
	colon := strings.Index(value, ":")
	return at >= 0 && colon > at
}

// RepoRef is a repository, however it was written: a boilerplate's starting
// point, or the repository a template store lives in.
type RepoRef struct {
	// URL is set when the recipe gave one outright.
	URL string
	// Host, Group and Name are set when it gave the shorthand
	// `server:group/name`, resolved against the configured git servers.
	Host, Group, Name string
}

// ParseRepoRef reads a repository however it was written.
//
// A URL is taken as it is. Anything else is `server:group/name`, where the
// server is one of the configured gitServers, so that what names a repository
// can be shared between people whose remotes differ in scheme or host.
func ParseRepoRef(repo string) (RepoRef, error) {
	repo = strings.TrimSpace(repo)
	if repo == "" {
		return RepoRef{}, fmt.Errorf("the reference is empty")
	}
	if IsGitURL(repo) {
		return RepoRef{URL: repo}, nil
	}

	host, rest, ok := strings.Cut(repo, ":")
	if !ok || strings.TrimSpace(host) == "" {
		return RepoRef{}, fmt.Errorf("the reference %q is neither a URL nor server:group/name", repo)
	}
	group, name, ok := strings.Cut(strings.Trim(rest, "/"), "/")
	if !ok || strings.TrimSpace(group) == "" || strings.TrimSpace(name) == "" {
		return RepoRef{}, fmt.Errorf("the reference %q is missing the group or the repository", repo)
	}

	return RepoRef{Host: host, Group: group, Name: strings.TrimSuffix(name, ".git")}, nil
}

// String describes the reference, for a listing.
func (r RepoRef) String() string {
	if r.URL != "" {
		return r.URL
	}
	return fmt.Sprintf("%s:%s/%s", r.Host, r.Group, r.Name)
}

// URLs returns the URLs to clone from, the preferred one first.
func (r RepoRef) URLs() (primary, fallback string, err error) {
	if r.URL != "" {
		return r.URL, "", nil
	}
	return GitURLs(r.Host, r.Group, r.Name)
}

// CloneInto clones a repository into a folder, shallowly and at one ref.
//
// Shallow because a starting point is wanted for its files, not its history;
// what happens to the history afterwards is the caller's business.
func CloneInto(url, ref, target string) error {
	return clone(url, ref, target, true)
}

// CloneFull clones a repository with its history, for a folder that is going
// to be worked in and pushed from rather than copied out of.
func CloneFull(url, ref, target string) error {
	return clone(url, ref, target, false)
}

// clone is the one place git clone is spelled out.
func clone(url, ref, target string, shallow bool) error {
	args := []string{"clone", "--quiet"}
	if shallow {
		args = append(args, "--depth", "1")
	}
	if strings.TrimSpace(ref) != "" {
		args = append(args, "--branch", ref)
	}
	args = append(args, url, target)

	cli.Debug("Cloning %s into %s", url, target)
	if err := runGitIn(".", args...); err != nil {
		// A clone that failed halfway leaves a folder nobody asked for.
		if entries, readErr := os.ReadDir(target); readErr == nil && len(entries) == 0 {
			_ = os.Remove(target)
		}
		return fmt.Errorf("cannot clone %s: %w", url, err)
	}
	return nil
}

// CloneWithFallback clones from the first URL, and from the second when that
// fails — the same two-URL dance `folder sync` does.
func CloneWithFallback(primary, fallback, ref, target string) error {
	err := CloneInto(primary, ref, target)
	if err == nil || strings.TrimSpace(fallback) == "" {
		return err
	}

	cli.Debug("Cloning %s failed, trying %s", primary, fallback)
	if fallbackErr := CloneInto(fallback, ref, target); fallbackErr != nil {
		return fmt.Errorf("%w; and %v", err, fallbackErr)
	}
	return nil
}

// InitRepo makes a folder a git repository when it is not one yet.
func InitRepo(path, branch string) error {
	if IsGitRepo(path) {
		return nil
	}

	args := []string{"init", "--quiet"}
	if strings.TrimSpace(branch) != "" {
		args = append(args, "--initial-branch", branch)
	}
	if err := runGit(path, args...); err != nil {
		return fmt.Errorf("cannot init a repository in %s: %w", path, err)
	}
	return nil
}

// SetRemote adds a remote, or repoints it when it is already there.
func SetRemote(path, name, url string) error {
	remotes, err := gitRemotes(path)
	if err != nil {
		return fmt.Errorf("cannot read the remotes of %s: %w", path, err)
	}

	if _, exists := remotes[name]; exists {
		if err := runGit(path, "remote", "set-url", name, url); err != nil {
			return fmt.Errorf("cannot repoint %s: %w", name, err)
		}
		return nil
	}
	if err := runGit(path, "remote", "add", name, url); err != nil {
		return fmt.Errorf("cannot add the remote %s: %w", name, err)
	}
	return nil
}

// RemoteURL returns where a remote of a repository points, or an empty string
// when it has no such remote.
func RemoteURL(path, name string) string {
	remotes, err := gitRemotes(path)
	if err != nil {
		return ""
	}
	return remotes[name]
}

// Pull brings a repository up to date, fast-forward only.
//
// Only: a repository with local work is something to sort out by hand, and a
// merge nobody asked for is worse than a message saying so.
func Pull(path string) error {
	if err := runGit(path, "pull", "--quiet", "--ff-only"); err != nil {
		return fmt.Errorf("cannot update %s: %w", path, err)
	}
	return nil
}

// runGitIn runs git in a folder that may not exist yet, which `clone` needs:
// the usual helper passes -C, and -C wants somewhere to stand.
func runGitIn(dir string, args ...string) error {
	return runGit(dir, args...)
}
