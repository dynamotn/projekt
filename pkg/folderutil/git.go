package folderutil

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"gitlab.com/dynamo.foss/projekt/pkg/cli"
	"gitlab.com/dynamo.foss/projekt/pkg/lazypath"
)

func getGitServer(hostName string) *lazypath.GitServer {
	c := lazypath.GetConfig()
	for _, server := range c.GitServers {
		if server.Name == hostName {
			return &server
		}
	}
	return nil
}

// SyncGitRepos synchronizes all Git repositories in the configuration.
//
// One failing folder does not stop the others: every error is reported and the
// combined failure is returned at the end.
func SyncGitRepos(dryRun bool) error {
	c := lazypath.GetConfig()

	var errs []error
	for _, folder := range c.Folders {
		if folder.Git == nil {
			continue
		}

		if err := syncFolderGitRepos(folder, dryRun); err != nil {
			cli.Error("Failed to sync folder %s: %v", folder.Path, err)
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}

// CheckGitReposStatus checks status of all Git repositories
func CheckGitReposStatus() error {
	c := lazypath.GetConfig()

	var errs []error
	for _, folder := range c.Folders {
		if folder.Git == nil {
			continue
		}

		if err := checkFolderGitRepos(folder); err != nil {
			cli.Error("Failed to check folder %s: %v", folder.Path, err)
			errs = append(errs, err)
		}
	}

	return errors.Join(errs...)
}

func syncFolderGitRepos(folder lazypath.Folder, dryRun bool) error {
	if folder.Git == nil {
		return nil
	}

	// Ensure parent folder exists
	if err := os.MkdirAll(folder.Path, 0o755); err != nil {
		return fmt.Errorf("failed to create folder %s: %w", folder.Path, err)
	}

	gitServer := getGitServer(folder.Git.Host)
	if gitServer == nil {
		return fmt.Errorf("git server '%s' not found in configuration", folder.Git.Host)
	}

	var errs []error
	for _, repo := range folder.Git.Repos {
		if repo.Name == "" {
			cli.Warn("Skipping repo with empty name in folder %s", folder.Path)
			continue
		}
		repoPath := repoTargetPath(folder, repo)

		if _, err := os.Stat(repoPath); os.IsNotExist(err) {
			// Repository doesn't exist, clone it
			if dryRun {
				primaryURL, fallbackURL := getGitURLs(gitServer, folder.Git.Group, repo.Name)
				if fallbackURL != "" {
					cli.Info("[DRY RUN] Would clone: %s (fallback: %s) -> %s", primaryURL, fallbackURL, repoPath)
				} else {
					cli.Info("[DRY RUN] Would clone: %s -> %s", primaryURL, repoPath)
				}
				continue
			}

			cli.Info("Cloning %s to %s", repo.Name, repoPath)
			if err := cloneRepoWithFallback(gitServer, folder.Git.Group, repo.Name, repoPath); err != nil {
				cli.Error("Failed to clone %s: %v", repo.Name, err)
				// Keep going with the other repos, but remember the failure so
				// the command exits non-zero.
				errs = append(errs, fmt.Errorf("clone %s: %w", repo.Name, err))
				continue
			}
			cli.Info("Successfully cloned %s", repo.Name)
		} else {
			// Repository exists, check if it's valid
			if dryRun {
				cli.Info("[DRY RUN] Would check: %s", repoPath)
				continue
			}

			cli.Debug("Repository %s already exists at %s", repo.Name, repoPath)
		}
	}

	return errors.Join(errs...)
}

func checkFolderGitRepos(folder lazypath.Folder) error {
	if folder.Git == nil {
		return nil
	}

	cli.Info("\nChecking folder: %s", folder.Path)
	cli.Info("  Git Host: %s", folder.Git.Host)
	cli.Info("  Git Group: %s", folder.Git.Group)

	gitServer := getGitServer(folder.Git.Host)
	if gitServer == nil {
		cli.Warn("  Git server '%s' not found in configuration", folder.Git.Host)
		return nil
	}

	for _, repo := range folder.Git.Repos {
		repoPath := repoTargetPath(folder, repo)

		if _, err := os.Stat(repoPath); os.IsNotExist(err) {
			cli.Warn("  [MISSING] %s (%s)", repo.Name, repoPath)
		} else {
			// Check if it's a valid Git repository
			gitDir := filepath.Join(repoPath, ".git")
			if _, err := os.Stat(gitDir); os.IsNotExist(err) {
				cli.Warn("  [NOT GIT] %s (%s)", repo.Name, repoPath)
			} else {
				// Check remote URL
				if err := checkGitRemote(repoPath, gitServer, folder.Git, repo); err != nil {
					cli.Warn("  [WARNING] %s: %v", repo.Name, err)
				} else {
					cli.Info("  [OK] %s (%s)", repo.Name, repoPath)
				}
			}
		}
	}

	return nil
}

func buildGitURL(server *lazypath.GitServer, group string, repoName string) string {
	sshURL := strings.TrimSuffix(strings.TrimSpace(server.SSH), "/")
	repoPath := fmt.Sprintf("%s/%s.git", group, repoName)

	// An explicit ssh:// URL must keep its scheme: without it git reads
	// "host:port/path" as scp syntax and takes the port for a path element.
	if strings.HasPrefix(sshURL, "ssh://") {
		return fmt.Sprintf("%s/%s", sshURL, repoPath)
	}

	// Format: host or git@host[:port]
	if !strings.Contains(sshURL, "@") {
		sshURL = "git@" + sshURL
	}

	// scp syntax cannot express a port, so switch to a ssh:// URL when there is one.
	if host, port, ok := splitSSHPort(sshURL); ok {
		return fmt.Sprintf("ssh://%s:%s/%s", host, port, repoPath)
	}

	return fmt.Sprintf("%s:%s", sshURL, repoPath)
}

// splitSSHPort splits "git@host:2222" into "git@host" and "2222".
// It reports false when the part after the last colon is not a port number.
func splitSSHPort(sshURL string) (host string, port string, ok bool) {
	idx := strings.LastIndex(sshURL, ":")
	if idx < 0 {
		return "", "", false
	}

	host, port = sshURL[:idx], sshURL[idx+1:]
	if port == "" {
		return "", "", false
	}
	for _, r := range port {
		if r < '0' || r > '9' {
			return "", "", false
		}
	}

	return host, port, true
}

func buildHTTPSURL(server *lazypath.GitServer, group string, repoName string) string {
	return fmt.Sprintf("%s/%s/%s.git", strings.TrimSuffix(server.HTTPS, "/"), group, repoName)
}

// repoTargetPath returns where a repo is checked out inside its folder.
// An unset path defaults to the repo name instead of the folder itself.
func repoTargetPath(folder lazypath.Folder, repo lazypath.GitRepo) string {
	relPath := repo.Path
	if relPath == "" {
		relPath = repo.Name
	}
	return filepath.Join(folder.Path, relPath)
}

func getGitURLs(server *lazypath.GitServer, group string, repoName string) (primary string, fallback string) {
	sshURL := buildGitURL(server, group, repoName)
	httpsURL := buildHTTPSURL(server, group, repoName)

	if server.PreferGitSSH {
		return sshURL, httpsURL
	}
	// Fall back the other way around too, so a server reachable only over SSH
	// still works when HTTPS cloning fails (private repo, no credential helper).
	return httpsURL, sshURL
}

func cloneRepoWithFallback(server *lazypath.GitServer, group string, repoName string, targetPath string) error {
	primaryURL, fallbackURL := getGitURLs(server, group, repoName)

	// Try primary URL
	cli.Debug("Attempting to clone from: %s", primaryURL)
	err := cloneRepo(primaryURL, targetPath)
	if err == nil {
		return nil
	}

	// If primary failed and we have fallback, try it
	if fallbackURL != "" {
		cli.Warn("Failed to clone via %s, trying fallback %s",
			getURLType(primaryURL), getURLType(fallbackURL))
		cli.Debug("Attempting to clone from: %s", fallbackURL)
		return cloneRepo(fallbackURL, targetPath)
	}

	return err
}

func getURLType(url string) string {
	if strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://") {
		return "HTTPS"
	}
	return "SSH"
}

func cloneRepo(gitURL, targetPath string) error {
	cmd := exec.Command("git", "clone", gitURL, targetPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func checkGitRemote(repoPath string, server *lazypath.GitServer, gitConfig *lazypath.GitConfig, repo lazypath.GitRepo) error {
	cmd := exec.Command("git", "-C", repoPath, "remote", "get-url", "origin")
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("failed to get remote URL: %w", err)
	}

	remoteURL := strings.TrimSpace(string(output))
	expectedURL := buildGitURL(server, gitConfig.Group, repo.Name)

	// Also check HTTPS variant
	expectedHTTPS := fmt.Sprintf("%s/%s/%s.git", server.HTTPS, gitConfig.Group, repo.Name)

	if remoteURL != expectedURL && remoteURL != expectedHTTPS && !strings.Contains(remoteURL, repo.Name) {
		return fmt.Errorf("remote URL mismatch: got %s, expected %s", remoteURL, expectedURL)
	}

	return nil
}
