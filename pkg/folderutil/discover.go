package folderutil

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"gitlab.com/dynamo.foss/projekt/pkg/cli"
	"gitlab.com/dynamo.foss/projekt/pkg/lazypath"
)

// DiscoverRepos scans a workspace for repositories that are already checked out
// and returns the git configuration that would reproduce them.
//
// It exists because the configuration and the disk usually disagree in the
// boring direction: the repositories are already cloned, and writing them out
// by hand is what stops anyone from using `folder check` and `folder sync` at
// all.
//
// A folder's git section names one host and one group, so repositories from
// elsewhere cannot be represented. Those are reported and skipped rather than
// silently dropped.
func DiscoverRepos(folder lazypath.Folder, servers []lazypath.GitServer) (*lazypath.GitConfig, error) {
	if !folder.IsWorkspace {
		return nil, fmt.Errorf("discovery only works on a workspace")
	}
	if len(servers) == 0 {
		return nil, fmt.Errorf("no git servers configured: add a gitServers entry before discovering")
	}

	folderPath := filepath.Clean(folder.Path)
	re, err := regexp.Compile(folder.GetRegexMatch())
	if err != nil {
		return nil, fmt.Errorf("cannot compile regex for folder %s: %w", folder.Path, err)
	}
	entries, err := os.ReadDir(folderPath)
	if err != nil {
		return nil, fmt.Errorf("cannot read folder %s: %w", folder.Path, err)
	}

	found := make([]discovered, 0, len(entries))
	for _, entry := range entries {
		if !isDirOrLinkToDir(folderPath, entry) || !re.MatchString(entry.Name()) {
			continue
		}

		childPath := filepath.Join(folderPath, entry.Name())
		if _, err := os.Stat(filepath.Join(childPath, ".git")); err != nil {
			cli.Debug("Not a git repository, skipping: %s", childPath)
			continue
		}

		remoteURL, err := repoRemoteURL(childPath, "origin")
		if err != nil {
			cli.Warn("Skipping %s: %v", entry.Name(), err)
			continue
		}

		server, group, name, ok := matchRemoteToServer(servers, remoteURL)
		if !ok {
			cli.Warn("Skipping %s: remote %s matches no configured git server", entry.Name(), remoteURL)
			continue
		}

		found = append(found, discovered{
			host:    server,
			group:   group,
			name:    name,
			dirName: entry.Name(),
		})
	}

	if len(found) == 0 {
		return nil, fmt.Errorf("found no git repository under %s", folder.Path)
	}

	return buildDiscoveredConfig(found), nil
}

// discovered is one repository found on disk, already resolved against the
// configured servers.
type discovered struct {
	host    string
	group   string
	name    string
	dirName string
}

// buildDiscoveredConfig picks the host and group that most of the repositories
// share and collects those, because a folder can only name one of each.
func buildDiscoveredConfig(found []discovered) *lazypath.GitConfig {
	counts := make(map[string]int, len(found))
	for _, repo := range found {
		counts[repo.host+"\x00"+repo.group]++
	}

	// Sort the candidates so that an equal split resolves the same way twice,
	// rather than following map iteration order.
	keys := make([]string, 0, len(counts))
	for key := range counts {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if counts[keys[i]] != counts[keys[j]] {
			return counts[keys[i]] > counts[keys[j]]
		}
		return keys[i] < keys[j]
	})

	winner := keys[0]
	host, group, _ := strings.Cut(winner, "\x00")

	config := &lazypath.GitConfig{Host: host, Group: group}
	for _, repo := range found {
		if repo.host != host || repo.group != group {
			cli.Warn("Skipping %s: it lives in %s/%s, but this folder is being set up for %s/%s",
				repo.dirName, repo.host, repo.group, host, group)
			continue
		}

		entry := lazypath.GitRepo{Name: repo.name}
		// Only record a path when the checkout is not simply named after the
		// repository, which is the default sync already assumes.
		if repo.dirName != repo.name {
			entry.Path = repo.dirName
		}
		config.Repos = append(config.Repos, entry)

		cli.Info("Discovered %s (%s/%s)", repo.dirName, group, repo.name)
	}

	return config
}

// repoRemoteURL reads one remote's URL out of a checked-out repository.
func repoRemoteURL(repoPath string, remote string) (string, error) {
	cmd := exec.Command("git", "-C", repoPath, "remote", "get-url", remote)
	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("cannot read the %s remote: %w", remote, err)
	}

	url := strings.TrimSpace(string(output))
	if url == "" {
		return "", fmt.Errorf("the %s remote has no URL", remote)
	}
	return url, nil
}

// matchRemoteToServer finds which configured server a remote URL belongs to,
// and splits the rest of the URL into the group and the repository name.
//
// The longest matching prefix wins, so that a server configured at a path under
// another one is not shadowed by it.
func matchRemoteToServer(servers []lazypath.GitServer, remoteURL string) (host string, group string, name string, ok bool) {
	var bestPrefix string

	for _, server := range servers {
		for _, prefix := range serverURLPrefixes(&server) {
			if len(prefix) <= len(bestPrefix) || !strings.HasPrefix(remoteURL, prefix) {
				continue
			}

			rest := strings.TrimSuffix(strings.TrimPrefix(remoteURL, prefix), ".git")
			repoGroup, repoName, found := cutLast(rest, "/")
			if !found || repoGroup == "" || repoName == "" {
				continue
			}

			bestPrefix = prefix
			host, group, name, ok = server.Name, repoGroup, repoName, true
		}
	}

	return host, group, name, ok
}

// serverURLPrefixes lists the URL prefixes a server's repositories can start
// with. It mirrors buildGitURL and buildHTTPSURL, and additionally accepts the
// scp form of a server configured with a port (and the other way round), since
// an existing checkout may have been cloned by hand in either spelling.
func serverURLPrefixes(server *lazypath.GitServer) []string {
	var prefixes []string

	if https := strings.TrimSuffix(strings.TrimSpace(server.HTTPS), "/"); https != "" {
		prefixes = append(prefixes, https+"/")
	}

	sshURL := strings.TrimSuffix(strings.TrimSpace(server.SSH), "/")
	if sshURL == "" {
		return prefixes
	}
	if strings.HasPrefix(sshURL, "ssh://") {
		prefixes = append(prefixes, sshURL+"/")

		// The same server is reachable by the scp spelling, which is what a
		// checkout cloned by hand most likely used. That form cannot carry the
		// port, so drop it.
		authority := strings.TrimPrefix(sshURL, "ssh://")
		if hostOnly, _, hasPort := splitSSHPort(authority); hasPort {
			authority = hostOnly
		}
		return append(prefixes, authority+":")
	}

	if !strings.Contains(sshURL, "@") {
		sshURL = "git@" + sshURL
	}
	if hostOnly, port, hasPort := splitSSHPort(sshURL); hasPort {
		return append(prefixes,
			fmt.Sprintf("ssh://%s:%s/", hostOnly, port),
			hostOnly+":",
		)
	}

	return append(prefixes,
		sshURL+":",
		"ssh://"+sshURL+"/",
	)
}

// cutLast is strings.Cut around the last separator instead of the first, so
// that a nested group stays with the group rather than with the name.
func cutLast(s string, sep string) (before string, after string, found bool) {
	idx := strings.LastIndex(s, sep)
	if idx < 0 {
		return s, "", false
	}
	return s[:idx], s[idx+len(sep):], true
}
