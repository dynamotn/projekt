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

func TestRemoteToken_StaysWithItsHost(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	t.Setenv("GITHUB_TOKEN", "public")
	t.Setenv("GH_ENTERPRISE_TOKEN", "corp")
	t.Setenv("GH_HOST", "git.corp.example")
	t.Setenv("GITLAB_TOKEN", "lab")
	t.Setenv("GITLAB_HOST", "https://lab.corp.example")

	cases := []struct {
		server lazypath.GitServer
		want   string
	}{
		{lazypath.GitServer{Type: "github", HTTPS: "https://github.com"}, "public"},
		{lazypath.GitServer{Type: "github", HTTPS: "https://git.corp.example"}, "corp"},
		{lazypath.GitServer{Type: "gitlab", HTTPS: "https://gitlab.com"}, "lab"},
		{lazypath.GitServer{Type: "gitlab", HTTPS: "https://lab.corp.example/"}, "lab"},
		{lazypath.GitServer{Type: "gitlab", HTTPS: "https://other.example"}, ""},
		{lazypath.GitServer{Type: "github", HTTPS: "http://github.com"}, ""},
	}
	for _, tc := range cases {
		if got := remoteToken(tc.server); got != tc.want {
			t.Errorf("remoteToken(%s %s) = %q, want %q", tc.server.Type, tc.server.HTTPS, got, tc.want)
		}
	}
}

func TestDiscoverRemoteRepos_GitLabStopsAPageLoop(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Next-Page", "1")
		_ = json.NewEncoder(w).Encode([]map[string]any{{"path_with_namespace": "org/api"}})
	}))
	defer server.Close()

	servers := []lazypath.GitServer{{Name: "lab", Type: "gitlab", HTTPS: server.URL}}
	if _, err := DiscoverRemoteRepos(servers, RemoteDiscoverOptions{Host: "lab", Group: "org", Client: server.Client()}); err == nil {
		t.Error("DiscoverRemoteRepos() error = nil, want a page loop refused")
	}
}

func TestDiscoverRemoteRepos_SkipsNamesThatLeaveTheFolder(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode([]map[string]any{{"name": ".."}, {"name": "api"}})
	}))
	defer server.Close()

	servers := []lazypath.GitServer{{Name: "corp", Type: "github", HTTPS: server.URL}}
	got, err := DiscoverRemoteRepos(servers, RemoteDiscoverOptions{Host: "corp", Group: "org", Token: "x", Client: server.Client()})
	if err != nil {
		t.Fatalf("DiscoverRemoteRepos() error = %v", err)
	}
	if want := []lazypath.GitRepo{{Name: "api"}}; !reflect.DeepEqual(got.Repos, want) {
		t.Errorf("Repos = %+v, want %+v", got.Repos, want)
	}
}

func TestRemoteToken_EnterpriseNeedsItsHostNamed(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	t.Setenv("GH_ENTERPRISE_TOKEN", "corp")
	t.Setenv("GH_HOST", "")
	// Any server an included file declares would otherwise receive it.
	if got := remoteToken(lazypath.GitServer{Type: "github", HTTPS: "https://evil.example"}); got != "" {
		t.Errorf("remoteToken() = %q, want nothing without GH_HOST naming the server", got)
	}
}

func TestSameOriginRedirect(t *testing.T) {
	from, _ := http.NewRequest(http.MethodGet, "https://lab.example/api/v4/groups/x", nil)
	for target, ok := range map[string]bool{
		"https://lab.example/api/v4/groups/y": true,
		"http://lab.example/api/v4/groups/x":  false,
		"https://evil.example/collect":        false,
		"https://sub.lab.example/x":           false,
	} {
		to, _ := http.NewRequest(http.MethodGet, target, nil)
		if err := sameOriginRedirect(to, []*http.Request{from}); (err == nil) != ok {
			t.Errorf("sameOriginRedirect(%s) error = %v, want followed %v", target, err, ok)
		}
	}
}

func TestSafeRepoName(t *testing.T) {
	for name, ok := range map[string]bool{
		"api": true, "sub/web": true, "..": false, "a/../b": false, "/etc": false,
		".git": false, "sub/.GIT": false, "a\nb": false, "": false, "a//b": false,
	} {
		if got := safeRepoName(name); got != ok {
			t.Errorf("safeRepoName(%q) = %v, want %v", name, got, ok)
		}
	}
}
