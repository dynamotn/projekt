package folderutil

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

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

// DefaultSyncForks is the number of repositories cloned at once when
// SyncOptions.Forks is left unset.
const DefaultSyncForks = 4

// SyncOptions controls how SyncGitRepos runs.
type SyncOptions struct {
	// DryRun reports what would happen without cloning anything.
	DryRun bool
	// Forks is the maximum number of repositories cloned concurrently.
	// Zero or less selects DefaultSyncForks, and 1 restores sequential cloning.
	Forks int
	// Tags syncs only the folders carrying every one of these tags. Empty syncs
	// everything.
	Tags []string
}

// syncJob is one repository to clone, resolved before the concurrent phase so
// that the workers never read the shared configuration.
type syncJob struct {
	group    string
	server   *lazypath.GitServer
	repo     lazypath.GitRepo
	repoPath string
	// worktreePaths is where each of repo.Worktrees belongs, resolved against
	// the folder while the configuration is still being read.
	worktreePaths []string
}

// SyncGitRepos synchronizes all Git repositories in the configuration.
//
// Repositories are cloned concurrently, at most SyncOptions.Forks at a time.
// One failing repository does not stop the others: every error is reported and
// the combined failure is returned at the end, in configuration order.
func SyncGitRepos(opts SyncOptions) error {
	c := lazypath.GetConfig()

	var errs []error
	var jobs []syncJob
	for _, folder := range c.Folders {
		if folder.Git == nil {
			continue
		}
		if !lazypath.HasTags(folder.Tags, opts.Tags) {
			continue
		}

		folderJobs, err := planFolderSync(folder)
		if err != nil {
			cli.Error("Failed to sync folder %s: %v", folder.Path, err)
			errs = append(errs, err)
			continue
		}
		jobs = append(jobs, folderJobs...)
	}

	errs = append(errs, runSyncJobs(jobs, opts)...)

	return errors.Join(errs...)
}

// planFolderSync creates a folder and resolves the repositories to clone into
// it. This stays sequential: it reads the configuration and creates the shared
// parent directory, neither of which is worth doing concurrently.
func planFolderSync(folder lazypath.Folder) ([]syncJob, error) {
	// Ensure parent folder exists
	if err := os.MkdirAll(folder.Path, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create folder %s: %w", folder.Path, err)
	}

	gitServer := getGitServer(folder.Git.Host)
	if gitServer == nil {
		return nil, fmt.Errorf("git server '%s' not found in configuration", folder.Git.Host)
	}

	var jobs []syncJob
	for _, repo := range folder.Git.Repos {
		if repo.Name == "" {
			cli.Warn("Skipping repo with empty name in folder %s", folder.Path)
			continue
		}

		worktreePaths := make([]string, len(repo.Worktrees))
		for i, worktree := range repo.Worktrees {
			worktreePaths[i] = worktreeTargetPath(folder, worktree)
		}

		jobs = append(jobs, syncJob{
			group:         folder.Git.Group,
			server:        gitServer,
			repo:          repo,
			repoPath:      repoTargetPath(folder, repo),
			worktreePaths: worktreePaths,
		})
	}

	return jobs, nil
}

// effectiveForks resolves how many workers a run should use, given its options
// and how much work there is to do.
func effectiveForks(opts SyncOptions, jobCount int) int {
	// A dry run only prints what it would do, and reads better when the lines
	// come out in configuration order, which one worker guarantees.
	if opts.DryRun {
		return 1
	}

	forks := opts.Forks
	if forks <= 0 {
		forks = DefaultSyncForks
	}
	// More workers than repositories only adds idle goroutines.
	if forks > jobCount {
		forks = jobCount
	}

	return forks
}

// runSyncJobs clones the planned repositories, at most forks at a time, and
// returns the failures in job order so a run's outcome does not depend on which
// worker happened to finish first.
func runSyncJobs(jobs []syncJob, opts SyncOptions) []error {
	return runSyncJobsWith(jobs, opts, syncRepo)
}

// runSyncJobsWith is runSyncJobs with the per-repository step injected, so that
// tests can observe the order and concurrency the pool actually produces.
func runSyncJobsWith(jobs []syncJob, opts SyncOptions, run func(syncJob, bool) error) []error {
	if len(jobs) == 0 {
		return nil
	}

	forks := effectiveForks(opts, len(jobs))

	// Indexed by job so that no two workers share a slot and the errors keep
	// their configuration order.
	results := make([]error, len(jobs))

	if forks == 1 {
		// Run inline rather than through a one-slot pool: goroutines do not
		// queue on a semaphore in the order they were started, so a pool of one
		// would still log the repositories in an arbitrary order.
		for i, job := range jobs {
			results[i] = run(job, opts.DryRun)
		}
	} else {
		sem := make(chan struct{}, forks)
		var wg sync.WaitGroup

		for i, job := range jobs {
			wg.Add(1)
			go func() {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()

				results[i] = run(job, opts.DryRun)
			}()
		}
		wg.Wait()
	}

	var errs []error
	for _, err := range results {
		if err != nil {
			errs = append(errs, err)
		}
	}

	return errs
}

// syncRepo brings one repository in line with its configuration: cloned when
// missing, then its extra remotes and worktrees in place. Several goroutines
// run it at once, so it must touch nothing outside its own job.
func syncRepo(job syncJob, dryRun bool) error {
	if err := cloneIfMissing(job, dryRun); err != nil {
		return err
	}

	// Remotes and worktrees are reconciled on every run, not only right after
	// cloning: adding one to the configuration of a repository that is already
	// on disk is the ordinary way to get it.
	return errors.Join(
		ensureRemotes(job, dryRun),
		ensureWorktrees(job, dryRun),
	)
}

func cloneIfMissing(job syncJob, dryRun bool) error {
	if _, err := os.Stat(job.repoPath); !os.IsNotExist(err) {
		// Repository exists, check if it's valid
		if dryRun {
			cli.Info("[DRY RUN] Would check: %s", job.repoPath)
			return nil
		}

		cli.Debug("Repository %s already exists at %s", job.repo.Name, job.repoPath)
		return nil
	}

	// Repository doesn't exist, clone it
	if dryRun {
		primaryURL, fallbackURL := getGitURLs(job.server, job.group, job.repo.Name)
		if fallbackURL != "" {
			cli.Info("[DRY RUN] Would clone: %s (fallback: %s) -> %s", primaryURL, fallbackURL, job.repoPath)
		} else {
			cli.Info("[DRY RUN] Would clone: %s -> %s", primaryURL, job.repoPath)
		}
		return nil
	}

	cli.Info("Cloning %s to %s", job.repo.Name, job.repoPath)
	if err := cloneRepoWithFallback(job.server, job.group, job.repo.Name, job.repoPath); err != nil {
		cli.Error("Failed to clone %s: %v", job.repo.Name, err)
		// Keep going with the other repos, but remember the failure so the
		// command exits non-zero.
		return fmt.Errorf("clone %s: %w", job.repo.Name, err)
	}
	cli.Info("Successfully cloned %s", job.repo.Name)

	return nil
}

// ensureRemotes adds the configured remotes that are missing and repoints the
// ones aiming somewhere else.
func ensureRemotes(job syncJob, dryRun bool) error {
	names := job.repo.RemoteNames()
	if len(names) == 0 {
		return nil
	}

	if dryRun {
		for _, name := range names {
			cli.Info("[DRY RUN] Would set remote %s -> %s in %s", name, job.repo.Remotes[name], job.repoPath)
		}
		return nil
	}

	existing, err := gitRemotes(job.repoPath)
	if err != nil {
		return fmt.Errorf("read remotes of %s: %w", job.repo.Name, err)
	}

	var errs []error
	for _, name := range names {
		want := job.repo.Remotes[name]
		current, present := existing[name]

		switch {
		case !present:
			if err := runGit(job.repoPath, "remote", "add", name, want); err != nil {
				errs = append(errs, fmt.Errorf("add remote %s to %s: %w", name, job.repo.Name, err))
				continue
			}
			cli.Info("Added remote %s -> %s in %s", name, want, job.repo.Name)
		case current != want:
			if err := runGit(job.repoPath, "remote", "set-url", name, want); err != nil {
				errs = append(errs, fmt.Errorf("update remote %s of %s: %w", name, job.repo.Name, err))
				continue
			}
			cli.Info("Updated remote %s -> %s in %s", name, want, job.repo.Name)
		default:
			cli.Debug("Remote %s of %s already points at %s", name, job.repo.Name, want)
		}
	}

	return errors.Join(errs...)
}

// ensureWorktrees creates the configured working trees that are not there yet.
// An existing one is left alone: it may well have work in progress in it.
func ensureWorktrees(job syncJob, dryRun bool) error {
	var errs []error

	for i, worktree := range job.repo.Worktrees {
		path := job.worktreePaths[i]

		if strings.TrimSpace(worktree.Branch) == "" {
			cli.Warn("Skipping worktree %s of %s: no branch configured", path, job.repo.Name)
			continue
		}
		if dryRun {
			cli.Info("[DRY RUN] Would add worktree %s (%s) for %s", path, worktree.Branch, job.repo.Name)
			continue
		}
		if _, err := os.Stat(path); err == nil {
			cli.Debug("Worktree %s of %s already exists", path, job.repo.Name)
			continue
		}

		if err := addWorktree(job.repoPath, path, worktree.Branch); err != nil {
			errs = append(errs, fmt.Errorf("add worktree %s of %s: %w", path, job.repo.Name, err))
			continue
		}
		cli.Info("Added worktree %s (%s) for %s", path, worktree.Branch, job.repo.Name)
	}

	return errors.Join(errs...)
}

func addWorktree(repoPath, worktreePath, branch string) error {
	// `git worktree add <path> <branch>` requires the branch to exist already,
	// and -b creates it. Checking first means an existing branch keeps its
	// history instead of the command failing.
	if branchExists(repoPath, branch) {
		return runGit(repoPath, "worktree", "add", worktreePath, branch)
	}
	return runGit(repoPath, "worktree", "add", "-b", branch, worktreePath)
}

func branchExists(repoPath, branch string) bool {
	return runGit(repoPath, "rev-parse", "--verify", "--quiet", "refs/heads/"+branch) == nil
}

// CheckOptions controls which folders CheckGitReposStatus reports on.
type CheckOptions struct {
	// Tags checks only the folders carrying every one of these tags. Empty
	// checks everything.
	Tags []string
}

// CheckGitReposStatus checks status of all Git repositories
func CheckGitReposStatus(opts CheckOptions) error {
	c := lazypath.GetConfig()

	var errs []error
	for _, folder := range c.Folders {
		if folder.Git == nil {
			continue
		}
		if !lazypath.HasTags(folder.Tags, opts.Tags) {
			continue
		}

		if err := checkFolderGitRepos(folder); err != nil {
			cli.Error("Failed to check folder %s: %v", folder.Path, err)
			errs = append(errs, err)
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
			continue
		}

		// Check if it's a valid Git repository
		gitDir := filepath.Join(repoPath, ".git")
		if _, err := os.Stat(gitDir); os.IsNotExist(err) {
			cli.Warn("  [NOT GIT] %s (%s)", repo.Name, repoPath)
			continue
		}

		// Check remote URL
		if err := checkGitRemote(repoPath, gitServer, folder.Git, repo); err != nil {
			cli.Warn("  [WARNING] %s: %v", repo.Name, err)
		} else {
			cli.Info("  [OK] %s (%s)", repo.Name, repoPath)
		}

		checkExtraRemotes(repoPath, repo)
		checkWorktrees(folder, repo)
	}

	return nil
}

// checkExtraRemotes reports on the remotes the configuration asks for beyond
// the one the clone set up.
func checkExtraRemotes(repoPath string, repo lazypath.GitRepo) {
	names := repo.RemoteNames()
	if len(names) == 0 {
		return
	}

	existing, err := gitRemotes(repoPath)
	if err != nil {
		cli.Warn("    [WARNING] %s: cannot read remotes: %v", repo.Name, err)
		return
	}

	for _, name := range names {
		want := repo.Remotes[name]
		switch current, present := existing[name]; {
		case !present:
			cli.Warn("    [REMOTE MISSING] %s -> %s", name, want)
		case current != want:
			cli.Warn("    [REMOTE MISMATCH] %s: got %s, expected %s", name, current, want)
		default:
			cli.Info("    [REMOTE OK] %s -> %s", name, want)
		}
	}
}

// checkWorktrees reports which configured working trees are not on disk.
func checkWorktrees(folder lazypath.Folder, repo lazypath.GitRepo) {
	for _, worktree := range repo.Worktrees {
		path := worktreeTargetPath(folder, worktree)

		if _, err := os.Stat(path); os.IsNotExist(err) {
			cli.Warn("    [WORKTREE MISSING] %s (%s)", path, worktree.Branch)
			continue
		}
		cli.Info("    [WORKTREE OK] %s (%s)", path, worktree.Branch)
	}
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

// worktreeTargetPath returns where an extra working tree belongs. A relative
// path is resolved against the folder, like a repo's own path, so that a
// worktree ends up beside the repositories rather than inside one of them.
func worktreeTargetPath(folder lazypath.Folder, worktree lazypath.GitWorktree) string {
	if filepath.IsAbs(worktree.Path) {
		return filepath.Clean(worktree.Path)
	}
	return filepath.Join(folder.Path, worktree.Path)
}

// runGit runs a git command in a repository, discarding its output. Concurrent
// jobs would interleave it on the shared streams, and the error carries what
// went wrong anyway.
func runGit(repoPath string, args ...string) error {
	cmd := exec.Command("git", append([]string{"-C", repoPath}, args...)...)

	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output

	if err := cmd.Run(); err != nil {
		if text := strings.TrimSpace(output.String()); text != "" {
			return fmt.Errorf("%w: %s", err, text)
		}
		return err
	}

	return nil
}

// gitRemotes reads the remotes configured on a repository, by name.
func gitRemotes(repoPath string) (map[string]string, error) {
	cmd := exec.Command("git", "-C", repoPath, "remote", "-v")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list remotes: %w", err)
	}

	// Each remote appears twice, as "name\turl (fetch)" and "(push)". The fetch
	// URL is the one a clone and a set-url agree on, so take that one.
	remotes := make(map[string]string)
	for _, line := range strings.Split(string(output), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 3 || fields[2] != "(fetch)" {
			continue
		}
		remotes[fields[0]] = fields[1]
	}

	return remotes, nil
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
		// A failed clone can leave an empty directory behind, which would make
		// the retry fail with "destination path already exists".
		if entries, readErr := os.ReadDir(targetPath); readErr == nil && len(entries) == 0 {
			_ = os.Remove(targetPath)
		}

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
	// Concurrent clones would interleave git's progress output line by line on
	// the shared streams, so buffer it per clone and surface it as a whole:
	// attached to the error when the clone fails, and only at debug level when
	// it succeeds.
	var output bytes.Buffer
	cmd.Stdout = &output
	cmd.Stderr = &output

	if err := cmd.Run(); err != nil {
		if text := strings.TrimSpace(output.String()); text != "" {
			return fmt.Errorf("%w: %s", err, text)
		}
		return err
	}

	cli.Debug("git clone %s:\n%s", gitURL, strings.TrimSpace(output.String()))

	return nil
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
