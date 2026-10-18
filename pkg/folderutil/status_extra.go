package folderutil

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"gitlab.com/dynamo-tools/projekt/pkg/cli"
	"gitlab.com/dynamo-tools/projekt/pkg/lazypath"
)

// ParseSince reads the durations `--since` takes: Go's own (`12h`, `90m`) and
// the calendar ones a person types (`7d`, `2w`, `3mo`, `1y`).
func ParseSince(value string) (time.Duration, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, nil
	}
	units := map[string]time.Duration{
		"d": 24 * time.Hour, "w": 7 * 24 * time.Hour,
		"mo": 30 * 24 * time.Hour, "y": 365 * 24 * time.Hour,
	}
	for suffix, unit := range units {
		if number, ok := strings.CutSuffix(value, suffix); ok {
			if n, err := strconv.ParseFloat(number, 64); err == nil && n >= 0 {
				return time.Duration(n * float64(unit)), nil
			}
		}
	}
	d, err := time.ParseDuration(value)
	if err != nil || d < 0 {
		return 0, fmt.Errorf("--since %q: use something like 7d, 2w, 3mo or 12h", value)
	}
	return d, nil
}

// touchedSince reports whether the folder had a commit, or was jumped to,
// after the cutoff.
func (s FolderStatus) touchedSince(cutoff time.Time, lastJump map[string]time.Time) bool {
	if !s.committedAt.IsZero() && !s.committedAt.Before(cutoff) {
		return true
	}
	if at, ok := lastJump[lazypath.Canonical(s.Path)]; ok && !at.Before(cutoff) {
		return true
	}
	return false
}

// jumpTimes reads when each project was last jumped to, by path.
func jumpTimes() map[string]time.Time {
	visits := lazypath.ReadHistory()
	times := make(map[string]time.Time, len(visits))
	for _, visit := range visits {
		times[lazypath.Canonical(visit.Path)] = visit.At
	}
	return times
}

// readCommittedAt reads when the last commit was made.
func readCommittedAt(path string) time.Time {
	out, err := gitOutput(path, "log", "-1", "--format=%ct")
	if err != nil {
		return time.Time{}
	}
	seconds, err := strconv.ParseInt(strings.TrimSpace(out), 10, 64)
	if err != nil {
		return time.Time{}
	}
	return time.Unix(seconds, 0)
}

// readBranchStanding compares the checked-out branch with the default one:
// how many commits of the default branch it has not got, and how long ago it
// parted from it. On the default branch itself both are empty.
func readBranchStanding(status *FolderStatus) {
	base := defaultBranch(status.Path)
	if base == "" {
		return
	}
	status.DefaultBranch = strings.TrimPrefix(base, "origin/")

	head, err := gitOutput(status.Path, "rev-parse", "HEAD")
	if err != nil {
		return
	}
	tip, err := gitOutput(status.Path, "rev-parse", base)
	if err != nil {
		return
	}
	if strings.TrimSpace(head) == strings.TrimSpace(tip) || status.Branch == status.DefaultBranch {
		return
	}

	if out, err := gitOutput(status.Path, "rev-list", "--count", "HEAD.."+base); err == nil {
		status.BehindDefault, _ = strconv.Atoi(strings.TrimSpace(out))
	}
	forkPoint, err := gitOutput(status.Path, "merge-base", "HEAD", base)
	if err != nil {
		return
	}
	if out, err := gitOutput(status.Path, "log", "-1", "--format=%cr", strings.TrimSpace(forkPoint)); err == nil {
		status.BranchAge = strings.TrimSpace(out)
	}
}

// defaultBranch names the branch a repository considers its main line:
// what origin says its HEAD is, or failing that a local main or master.
func defaultBranch(path string) string {
	if out, err := gitOutput(path, "symbolic-ref", "--quiet", "--short", "refs/remotes/origin/HEAD"); err == nil {
		if name := strings.TrimSpace(out); name != "" {
			return name
		}
	}
	for _, candidate := range []string{"origin/main", "origin/master", "main", "master"} {
		if runGit(path, "rev-parse", "--verify", "--quiet", candidate) == nil {
			return candidate
		}
	}
	return ""
}

// The answers the requests column gives when it cannot count.
const (
	requestsNoTool   = "n/a"
	requestsUnknown  = "?"
	requestsNoBranch = ""
)

// readOpenRequests counts the open merge or pull requests of the checked-out
// branch, through the server's own command-line tool: `glab` for a GitLab
// origin, `gh` for a GitHub one. No tool on PATH is "n/a"; a tool that fails
// — not logged in, no network — is "?".
func readOpenRequests(status *FolderStatus, servers []lazypath.GitServer) {
	if status.Branch == "" || status.Branch == "detached" {
		status.Requests = requestsNoBranch
		return
	}
	origin, err := repoRemoteURL(status.Path, "origin")
	if err != nil {
		status.Requests = requestsNoBranch
		return
	}

	var tool []string
	switch requestTool(origin, servers) {
	case "gitlab":
		tool = []string{"glab", "mr", "list", "--source-branch", status.Branch, "--output", "json"}
	case "github":
		tool = []string{"gh", "pr", "list", "--head", status.Branch, "--state", "open", "--json", "number"}
	default:
		status.Requests = requestsNoTool
		return
	}
	if _, err := exec.LookPath(tool[0]); err != nil {
		status.Requests = requestsNoTool
		return
	}

	cmd := exec.Command(tool[0], tool[1:]...)
	cmd.Dir = status.Path
	out, err := cmd.Output()
	if err != nil {
		cli.Debug("%s in %s: %v", strings.Join(tool, " "), status.Path, err)
		status.Requests = requestsUnknown
		return
	}
	var items []json.RawMessage
	if err := json.Unmarshal(out, &items); err != nil {
		status.Requests = requestsUnknown
		return
	}
	status.Requests = strconv.Itoa(len(items))
}

// requestTool says which server family an origin belongs to: the configured
// server whose URL it starts with, or failing that the host name itself.
func requestTool(origin string, servers []lazypath.GitServer) string {
	if host, _, _, ok := matchRemoteToServer(servers, origin); ok {
		for _, server := range servers {
			if server.Name == host {
				return strings.ToLower(strings.TrimSpace(server.Type))
			}
		}
	}
	lower := strings.ToLower(origin)
	switch {
	case strings.Contains(lower, "gitlab"):
		return "gitlab"
	case strings.Contains(lower, "github"):
		return "github"
	}
	return ""
}
