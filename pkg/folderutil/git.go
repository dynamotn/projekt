package folderutil

import (
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

// SyncGitRepos synchronizes all Git repositories in the configuration
func SyncGitRepos(dryRun bool) error {
	c := lazypath.GetConfig()

	for _, folder := range c.Folders {
		if folder.Git == nil {
			continue
		}

		if err := syncFolderGitRepos(folder, dryRun); err != nil {
			cli.Error("Failed to sync folder %s: %v", folder.Path, err)
			return err
		}
	}

	return nil
}

// CheckGitReposStatus checks status of all Git repositories
func CheckGitReposStatus() error {
	c := lazypath.GetConfig()

	for _, folder := range c.Folders {
		if folder.Git == nil {
			continue
		}

		if err := checkFolderGitRepos(folder); err != nil {
			cli.Error("Failed to check folder %s: %v", folder.Path, err)
			return err
		}
	}

	return nil
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

	for _, repo := range folder.Git.Repos {
		repoPath := filepath.Join(folder.Path, repo.Path)

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

	return nil
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
		repoPath := filepath.Join(folder.Path, repo.Path)

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
	// Parse SSH URL to extract host and port
	sshURL := server.SSH

	// Format: ssh://git@host:port or git@host:path
	if strings.HasPrefix(sshURL, "ssh://") {
		// Remove ssh:// prefix
		sshURL = strings.TrimPrefix(sshURL, "ssh://")
		// Format: git@host:port/path or git@host:port
		return fmt.Sprintf("%s/%s/%s.git", sshURL, group, repoName)
	}

	// If SSH URL is in git@host:path format
	if strings.Contains(sshURL, "@") {
		return fmt.Sprintf("%s:%s/%s.git", sshURL, group, repoName)
	}

	// Fallback: construct from scratch
	return fmt.Sprintf("git@%s:%s/%s.git", server.SSH, group, repoName)
}

func buildHTTPSURL(server *lazypath.GitServer, group string, repoName string) string {
	return fmt.Sprintf("%s/%s/%s.git", server.HTTPS, group, repoName)
}

func getGitURLs(server *lazypath.GitServer, group string, repoName string) (primary string, fallback string) {
	sshURL := buildGitURL(server, group, repoName)
	httpsURL := buildHTTPSURL(server, group, repoName)

	if server.PreferGitSSH {
		return sshURL, httpsURL
	}
	return httpsURL, ""
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
		return fmt.Errorf("failed to get remote URL")
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
