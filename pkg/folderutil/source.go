package folderutil

import (
	"fmt"
	"os"
	"strings"

	"gitlab.com/dynamo.foss/projekt/pkg/cli"
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

// CloneInto clones a repository into a folder, shallowly and at one ref.
//
// Shallow because a starting point is wanted for its files, not its history;
// what happens to the history afterwards is the caller's business.
func CloneInto(url, ref, target string) error {
	args := []string{"clone", "--quiet", "--depth", "1"}
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

// runGitIn runs git in a folder that may not exist yet, which `clone` needs:
// the usual helper passes -C, and -C wants somewhere to stand.
func runGitIn(dir string, args ...string) error {
	return runGit(dir, args...)
}
