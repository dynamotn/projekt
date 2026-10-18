package folderutil

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"gitlab.com/dynamo-tools/projekt/pkg/lazypath"
)

func TestDiscoverRemoteRepos_GitLab(t *testing.T) {
	var seenAuth []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seenAuth = append(seenAuth, r.Header.Get("Authorization"))
		if !strings.HasPrefix(r.URL.Path, "/api/v4/groups/myorg%2Fteam/projects") &&
			!strings.HasPrefix(r.URL.EscapedPath(), "/api/v4/groups/myorg%2Fteam/projects") {
			http.NotFound(w, r)
			return
		}
		if r.URL.Query().Get("include_subgroups") != "true" {
			t.Errorf("include_subgroups = %q, want true", r.URL.Query().Get("include_subgroups"))
		}
		switch r.URL.Query().Get("page") {
		case "1":
			w.Header().Set("X-Next-Page", "2")
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{"path_with_namespace": "myorg/team/api", "archived": false},
				{"path_with_namespace": "myorg/team/old", "archived": true},
			})
		case "2":
			w.Header().Set("X-Next-Page", "")
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{"path_with_namespace": "myorg/team/sub/web", "archived": false},
				{"path_with_namespace": "myorg/team/web", "archived": false},
			})
		}
	}))
	defer server.Close()

	servers := []lazypath.GitServer{{Name: "lab", Type: "gitlab", HTTPS: server.URL}}
	got, err := DiscoverRemoteRepos(servers, RemoteDiscoverOptions{
		Host: "lab", Group: "myorg/team", Recursive: true, Token: "secret", Client: server.Client(),
	})
	if err != nil {
		t.Fatalf("DiscoverRemoteRepos() error = %v", err)
	}
	if got.Host != "lab" || got.Group != "myorg/team" {
		t.Errorf("Host/Group = %q/%q", got.Host, got.Group)
	}
	// The archived one is skipped, and the subgroup's web collides with the
	// group's own web on disk, so it is reported and skipped.
	want := []lazypath.GitRepo{{Name: "api"}, {Name: "web"}}
	if !reflect.DeepEqual(got.Repos, want) {
		t.Errorf("Repos = %+v, want %+v", got.Repos, want)
	}
	for _, auth := range seenAuth {
		if auth != "Bearer secret" {
			t.Errorf("Authorization = %q, want the token sent", auth)
		}
	}
}

func TestDiscoverRemoteRepos_GitLabSubgroupsFlattenWhenDistinct(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode([]map[string]any{
			{"path_with_namespace": "org/sub/tool", "archived": false},
			{"path_with_namespace": "org/other", "archived": true},
		})
	}))
	defer server.Close()

	servers := []lazypath.GitServer{{Name: "lab", Type: "gitlab", HTTPS: server.URL}}
	got, err := DiscoverRemoteRepos(servers, RemoteDiscoverOptions{
		Host: "lab", Group: "org", Recursive: true, Archived: true, Client: server.Client(),
	})
	if err != nil {
		t.Fatalf("DiscoverRemoteRepos() error = %v", err)
	}
	want := []lazypath.GitRepo{{Name: "other"}, {Name: "sub/tool", Path: "tool"}}
	if !reflect.DeepEqual(got.Repos, want) {
		t.Errorf("Repos = %+v, want %+v", got.Repos, want)
	}
}

func TestDiscoverRemoteRepos_GitHubFallsBackToAUser(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasPrefix(r.URL.Path, "/api/v3/orgs/jane/"):
			http.NotFound(w, r)
		case strings.HasPrefix(r.URL.Path, "/api/v3/users/jane/repos"):
			_ = json.NewEncoder(w).Encode([]map[string]any{
				{"name": "dotfiles", "archived": false},
				{"name": "scratch", "archived": false},
				{"name": "attic", "archived": true},
			})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	servers := []lazypath.GitServer{{Name: "hub", Type: "github", HTTPS: server.URL}}
	got, err := DiscoverRemoteRepos(servers, RemoteDiscoverOptions{
		Host: "hub", Group: "jane", Exclude: "^scratch$", Client: server.Client(),
	})
	if err != nil {
		t.Fatalf("DiscoverRemoteRepos() error = %v", err)
	}
	want := []lazypath.GitRepo{{Name: "dotfiles"}}
	if !reflect.DeepEqual(got.Repos, want) {
		t.Errorf("Repos = %+v, want %+v", got.Repos, want)
	}
}

func TestDiscoverRemoteRepos_Errors(t *testing.T) {
	forbidden := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer forbidden.Close()
	servers := []lazypath.GitServer{
		{Name: "lab", Type: "gitlab", HTTPS: forbidden.URL},
		{Name: "odd", Type: "gitea", HTTPS: forbidden.URL},
	}

	cases := map[string]RemoteDiscoverOptions{
		"unknown host":    {Host: "nope", Group: "g"},
		"no group":        {Host: "lab"},
		"unknown type":    {Host: "odd", Group: "g"},
		"bad include":     {Host: "lab", Group: "g", Include: "("},
		"unauthenticated": {Host: "lab", Group: "g", Client: forbidden.Client()},
	}
	for name, o := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := DiscoverRemoteRepos(servers, o); err == nil {
				t.Error("DiscoverRemoteRepos() error = nil, want one")
			}
		})
	}
}

func TestGithubAPI(t *testing.T) {
	if got := githubAPI(lazypath.GitServer{HTTPS: "https://github.com"}); got != "https://api.github.com" {
		t.Errorf("githubAPI(github.com) = %q", got)
	}
	if got := githubAPI(lazypath.GitServer{HTTPS: "https://git.corp.example/"}); got != "https://git.corp.example/api/v3" {
		t.Errorf("githubAPI(enterprise) = %q", got)
	}
}

func TestRemoteToken_Environment(t *testing.T) {
	t.Setenv("GITLAB_TOKEN", "from-env")
	t.Setenv("PATH", t.TempDir())
	if got := remoteToken(lazypath.GitServer{Type: "gitlab", HTTPS: "https://gitlab.com"}); got != "from-env" {
		t.Errorf("remoteToken() = %q, want the environment's", got)
	}
	t.Setenv("GITLAB_TOKEN", "")
	if got := remoteToken(lazypath.GitServer{Type: "gitlab", HTTPS: "https://gitlab.com"}); got != "" {
		t.Errorf("remoteToken() = %q, want nothing without a tool on PATH", got)
	}
}
