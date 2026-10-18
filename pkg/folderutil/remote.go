package folderutil

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"gitlab.com/dynamo-tools/projekt/pkg/cli"
	"gitlab.com/dynamo-tools/projekt/pkg/lazypath"
)

// RemoteDiscoverOptions drives `folder add --from-remote`.
type RemoteDiscoverOptions struct {
	// Host names the configured git server, Group the group or organisation
	// on it whose repositories are wanted.
	Host, Group string
	// Recursive includes the repositories of subgroups too, which GitLab has
	// and GitHub does not.
	Recursive bool
	// Archived includes the repositories the server has archived.
	Archived bool
	// Include and Exclude are regular expressions on the repository name.
	Include, Exclude string
	// Token overrides the one found in the environment or the server's own
	// command-line tool. It is used for this request and kept nowhere.
	Token string
	// Client is the HTTP client, for a test; nil means a default one.
	Client *http.Client
}

// remoteRepo is one repository as a server lists it.
type remoteRepo struct {
	// Name is what the folder's git section calls it: the path under the
	// group, so a repository of a subgroup is `sub/name`.
	Name     string
	Archived bool
}

// DiscoverRemoteRepos asks a git server which repositories a group has, and
// returns the git section that would clone them.
//
// Nothing is on disk yet, which is the point: `folder add --from-remote`
// followed by `folder sync` is a new machine's whole setup. A token found in
// the environment or through `gh`/`glab` is sent along, and never stored.
func DiscoverRemoteRepos(servers []lazypath.GitServer, o RemoteDiscoverOptions) (*lazypath.GitConfig, error) {
	server, ok := findServer(servers, o.Host)
	if !ok {
		return nil, fmt.Errorf("git server %q is not configured", o.Host)
	}
	group := strings.Trim(strings.TrimSpace(o.Group), "/")
	if group == "" {
		return nil, errors.New("--group is required: the group or organisation to list")
	}
	if strings.TrimSpace(server.HTTPS) == "" {
		return nil, fmt.Errorf("git server %q has no https URL to call an API on", server.Name)
	}

	include, exclude, err := nameFilters(o.Include, o.Exclude)
	if err != nil {
		return nil, err
	}

	client := o.Client
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	token := o.Token
	if token == "" {
		token = remoteToken(server)
	}
	api := apiClient{client: client, token: token, server: server}

	var repos []remoteRepo
	switch strings.ToLower(strings.TrimSpace(server.Type)) {
	case "gitlab":
		repos, err = api.gitlabProjects(group, o.Recursive)
	case "github":
		if o.Recursive {
			cli.Warn("GitHub has no subgroups; --recursive changes nothing")
		}
		repos, err = api.githubRepos(group)
	default:
		return nil, fmt.Errorf("git server %q has type %q; --from-remote knows gitlab and github", server.Name, server.Type)
	}
	if err != nil {
		return nil, err
	}

	// The group's own repositories first, then the subgroups', so that when a
	// subgroup's repository would land on the same folder as the group's, it
	// is the subgroup's that gives way.
	sort.SliceStable(repos, func(i, j int) bool {
		di, dj := strings.Count(repos[i].Name, "/"), strings.Count(repos[j].Name, "/")
		if di != dj {
			return di < dj
		}
		return repos[i].Name < repos[j].Name
	})

	config := &lazypath.GitConfig{Host: server.Name, Group: group}
	taken := map[string]string{}
	for _, repo := range repos {
		switch {
		case repo.Archived && !o.Archived:
			cli.Debug("Skipping archived %s", repo.Name)
			continue
		case include != nil && !include.MatchString(repo.Name):
			cli.Debug("Skipping %s: does not match --include", repo.Name)
			continue
		case exclude != nil && exclude.MatchString(repo.Name):
			cli.Debug("Skipping %s: matches --exclude", repo.Name)
			continue
		}

		entry := lazypath.GitRepo{Name: repo.Name}
		// A subgroup's repository is cloned into the workspace under its own
		// name, beside the others, rather than nested under the subgroup.
		if base := path.Base(repo.Name); base != repo.Name {
			entry.Path = base
		}
		local := entry.Path
		if local == "" {
			local = entry.Name
		}
		if other, dup := taken[local]; dup {
			cli.Warn("Skipping %s: it would be checked out as %s, like %s", repo.Name, local, other)
			continue
		}
		taken[local] = repo.Name
		config.Repos = append(config.Repos, entry)
		cli.Info("Found %s (%s/%s)", local, group, repo.Name)
	}
	if len(config.Repos) == 0 {
		return nil, fmt.Errorf("found no repository in %s on %s", group, server.Name)
	}
	sort.Slice(config.Repos, func(i, j int) bool { return config.Repos[i].Name < config.Repos[j].Name })
	return config, nil
}

func findServer(servers []lazypath.GitServer, name string) (lazypath.GitServer, bool) {
	for _, server := range servers {
		if server.Name == name {
			return server, true
		}
	}
	return lazypath.GitServer{}, false
}

func nameFilters(include, exclude string) (in, out *regexp.Regexp, err error) {
	if strings.TrimSpace(include) != "" {
		if in, err = regexp.Compile(include); err != nil {
			return nil, nil, fmt.Errorf("--include: %w", err)
		}
	}
	if strings.TrimSpace(exclude) != "" {
		if out, err = regexp.Compile(exclude); err != nil {
			return nil, nil, fmt.Errorf("--exclude: %w", err)
		}
	}
	return in, out, nil
}

// remoteToken finds a token for a server without asking: the usual
// environment variables first, then the server's own command-line tool when
// it is on PATH and logged in. Nothing found means the request goes out
// unauthenticated, which a public group answers anyway.
func remoteToken(server lazypath.GitServer) string {
	host := apiHost(server)
	switch strings.ToLower(server.Type) {
	case "gitlab":
		for _, name := range []string{"GITLAB_TOKEN", "GL_TOKEN"} {
			if value := strings.TrimSpace(os.Getenv(name)); value != "" {
				return value
			}
		}
		return toolOutput("glab", "config", "get", "token", "--host", host)
	case "github":
		for _, name := range []string{"GITHUB_TOKEN", "GH_TOKEN"} {
			if value := strings.TrimSpace(os.Getenv(name)); value != "" {
				return value
			}
		}
		return toolOutput("gh", "auth", "token", "--hostname", host)
	}
	return ""
}

// toolOutput runs a command-line tool for one line of output, or returns ""
// when the tool is missing or unhappy. Its errors are not the user's problem:
// the request simply goes out without a token.
func toolOutput(name string, args ...string) string {
	if _, err := exec.LookPath(name); err != nil {
		return ""
	}
	out, err := exec.Command(name, args...).Output()
	if err != nil {
		cli.Debug("%s %v: %v", name, args, err)
		return ""
	}
	return strings.TrimSpace(string(out))
}

// apiHost is the host name a server's API lives on, for the tools' --host.
func apiHost(server lazypath.GitServer) string {
	parsed, err := url.Parse(strings.TrimSpace(server.HTTPS))
	if err != nil || parsed.Host == "" {
		return ""
	}
	return parsed.Hostname()
}

// apiClient talks to one server.
type apiClient struct {
	client *http.Client
	token  string
	server lazypath.GitServer
}

// get fetches one page of JSON and returns the response headers, which carry
// the pagination.
func (a apiClient) get(endpoint string, into any) (http.Header, error) {
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "projekt")
	if a.token != "" {
		req.Header.Set("Authorization", "Bearer "+a.token)
	}

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", cli.RedactURL(endpoint), err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("%s: not found — a private group needs a token, see --from-remote in the help", cli.RedactURL(endpoint))
	}
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return nil, fmt.Errorf("%s: %s — set GITLAB_TOKEN or GITHUB_TOKEN, or log in with glab or gh", cli.RedactURL(endpoint), resp.Status)
	}
	if resp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("%s: %s", cli.RedactURL(endpoint), resp.Status)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 32<<20))
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(body, into); err != nil {
		return nil, fmt.Errorf("%s: cannot read the answer: %w", cli.RedactURL(endpoint), err)
	}
	return resp.Header, nil
}

// gitlabProjects lists a group's projects, page by page.
func (a apiClient) gitlabProjects(group string, recursive bool) ([]remoteRepo, error) {
	base := strings.TrimSuffix(strings.TrimSpace(a.server.HTTPS), "/") + "/api/v4/groups/" + url.PathEscape(group) + "/projects"

	var repos []remoteRepo
	for page := 1; page > 0; {
		query := url.Values{
			"per_page":          {"100"},
			"page":              {strconv.Itoa(page)},
			"simple":            {"true"},
			"include_subgroups": {strconv.FormatBool(recursive)},
		}
		var items []struct {
			PathWithNamespace string `json:"path_with_namespace"`
			Archived          bool   `json:"archived"`
		}
		headers, err := a.get(base+"?"+query.Encode(), &items)
		if err != nil {
			return nil, err
		}
		for _, item := range items {
			name := strings.TrimPrefix(item.PathWithNamespace, group+"/")
			if name == "" || name == item.PathWithNamespace {
				cli.Debug("Skipping %s: not under %s", item.PathWithNamespace, group)
				continue
			}
			repos = append(repos, remoteRepo{Name: name, Archived: item.Archived})
		}
		page, _ = strconv.Atoi(strings.TrimSpace(headers.Get("X-Next-Page")))
	}
	return repos, nil
}

// githubRepos lists an organisation's repositories, or a user's when the
// name is not an organisation.
func (a apiClient) githubRepos(owner string) ([]remoteRepo, error) {
	api := githubAPI(a.server)
	var repos []remoteRepo
	for _, kind := range []string{"orgs", "users"} {
		found, err := a.githubPages(fmt.Sprintf("%s/%s/%s/repos", api, kind, url.PathEscape(owner)))
		if err == nil {
			return found, nil
		}
		if kind == "users" || !strings.Contains(err.Error(), "not found") {
			return nil, err
		}
		cli.Debug("%s is not an organisation, trying a user", owner)
	}
	return repos, nil
}

func (a apiClient) githubPages(base string) ([]remoteRepo, error) {
	var repos []remoteRepo
	for page := 1; ; page++ {
		var items []struct {
			Name     string `json:"name"`
			Archived bool   `json:"archived"`
		}
		query := url.Values{"per_page": {"100"}, "page": {strconv.Itoa(page)}, "type": {"all"}}
		if _, err := a.get(base+"?"+query.Encode(), &items); err != nil {
			return nil, err
		}
		for _, item := range items {
			repos = append(repos, remoteRepo{Name: item.Name, Archived: item.Archived})
		}
		if len(items) < 100 {
			return repos, nil
		}
	}
}

// githubAPI is where a GitHub server answers: api.github.com for the public
// one, /api/v3 under the server for an enterprise one.
func githubAPI(server lazypath.GitServer) string {
	https := strings.TrimSuffix(strings.TrimSpace(server.HTTPS), "/")
	if apiHost(server) == "github.com" {
		return "https://api.github.com"
	}
	return https + "/api/v3"
}
