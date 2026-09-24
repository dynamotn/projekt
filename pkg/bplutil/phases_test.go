package bplutil

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"gitlab.com/dynamo.foss/projekt/pkg/lazypath"
	"gitlab.com/dynamo.foss/projekt/pkg/tplutil"
)

func TestParseRepoRef(t *testing.T) {
	cases := map[string]RepoRef{
		"github:me/starter":              {Host: "github", Group: "me", Name: "starter"},
		"github:me/starter.git":          {Host: "github", Group: "me", Name: "starter"},
		"git@github.com:me/starter.git":  {URL: "git@github.com:me/starter.git"},
		"https://github.com/me/starter":  {URL: "https://github.com/me/starter"},
		"ssh://git@host:2222/me/starter": {URL: "ssh://git@host:2222/me/starter"},
		// A repository on this machine is somewhere git can clone from too.
		"/srv/starters/go.git": {URL: "/srv/starters/go.git"},
		"~/starters/go":        {URL: "~/starters/go"},
		"./starters/go":        {URL: "./starters/go"},
	}
	for input, want := range cases {
		got, err := ParseRepoRef(input)
		if err != nil {
			t.Errorf("ParseRepoRef(%q) error = %v", input, err)
			continue
		}
		if got != want {
			t.Errorf("ParseRepoRef(%q) = %#v, want %#v", input, got, want)
		}
	}

	for _, bad := range []string{"", "   ", "github:justaname", ":group/name", "github:/name", "github:group/"} {
		if _, err := ParseRepoRef(bad); err == nil {
			t.Errorf("ParseRepoRef(%q) error = nil, want an error", bad)
		}
	}
}

// bareOrigin makes a repository to clone from, with one file in it.
func bareOrigin(t *testing.T, content string) string {
	t.Helper()

	root := t.TempDir()
	work := filepath.Join(root, "work")
	bare := filepath.Join(root, "origin.git")

	run := func(dir string, args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}

	if err := os.MkdirAll(work, 0o755); err != nil {
		t.Fatal(err)
	}
	run(work, "init", "--quiet", "--initial-branch=main")
	run(work, "config", "user.email", "t@example.com")
	run(work, "config", "user.name", "T")
	if err := os.WriteFile(filepath.Join(work, "README.md"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	run(work, "add", "README.md")
	run(work, "commit", "--quiet", "-m", "first")
	run(root, "clone", "--quiet", "--bare", work, bare)

	return bare
}

func TestCreate_FromARepoVerbatim(t *testing.T) {
	recipes, templates := useStores(t)
	_ = templates
	origin := bareOrigin(t, "hello {{ .Values.name }}\n")

	write(t, filepath.Join(recipes, "starter.yaml"), "source:\n  repo: "+origin+"\n")
	recipe, err := Get("starter")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	target := filepath.Join(t.TempDir(), "myapp")
	var out bytes.Buffer
	result, err := Create(CreateOptions{Recipe: recipe, Target: target, NoRegister: true, Out: &out, Log: &out})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if len(result.Files) != 1 {
		t.Fatalf("files = %v, want the one file", result.Files)
	}

	// Verbatim: someone else's braces are their own.
	data, err := os.ReadFile(filepath.Join(target, "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "hello {{ .Values.name }}\n" {
		t.Errorf("README = %q, want it copied as it is", data)
	}
	// And the new project is not a fork of the starting point.
	if _, err := os.Stat(filepath.Join(target, ".git")); !os.IsNotExist(err) {
		t.Error("the clone's .git came along")
	}
}

func TestCreate_FromARepoRendered(t *testing.T) {
	recipes, _ := useStores(t)
	origin := bareOrigin(t, "hello {{ .Values.name }}\n")

	write(t, filepath.Join(recipes, "starter.yaml"), "source:\n  repo: "+origin+"\n  render: true\n")
	recipe, err := Get("starter")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	target := filepath.Join(t.TempDir(), "myapp")
	var out bytes.Buffer
	if _, err := Create(CreateOptions{
		Recipe:     recipe,
		Target:     target,
		Values:     tplutil.Values{"name": "world"},
		NoRegister: true,
		Out:        &out,
		Log:        &out,
	}); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	data, err := os.ReadFile(filepath.Join(target, "README.md"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "hello world\n" {
		t.Errorf("README = %q, want it rendered", data)
	}
}

func TestCreate_RunsAfterHooks(t *testing.T) {
	recipes, templates := useStores(t)
	write(t, filepath.Join(templates, "app", "go.mod.tmpl"), "module x\n")
	write(t, filepath.Join(recipes, "app.yaml"), `
source:
  template: app
after:
  - echo "made {{ .Name }}" > hook-ran
  - echo "$PROJEKT_NAME at $PROJEKT_PATH" > hook-env
`)
	recipe, err := Get("app")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	target := filepath.Join(t.TempDir(), "myapp")
	var out, hookOut bytes.Buffer
	if _, err := Create(CreateOptions{
		Recipe: recipe, Target: target, NoRegister: true,
		Out: &out, Log: &out, HookOut: &hookOut, HookErr: &hookOut,
	}); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// The command line is a template like anything else.
	ran, err := os.ReadFile(filepath.Join(target, "hook-ran"))
	if err != nil {
		t.Fatalf("the hook did not run: %v", err)
	}
	if strings.TrimSpace(string(ran)) != "made myapp" {
		t.Errorf("hook wrote %q", ran)
	}

	// And it knows where it is.
	env, err := os.ReadFile(filepath.Join(target, "hook-env"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(env), "myapp at "+target) {
		t.Errorf("hook environment = %q", env)
	}

	// Every command is printed before it runs.
	if !strings.Contains(out.String(), "Running: echo") {
		t.Errorf("out = %q, want the commands shown", out.String())
	}
}

func TestCreate_NoHooks(t *testing.T) {
	recipes, templates := useStores(t)
	write(t, filepath.Join(templates, "app", "go.mod.tmpl"), "module x\n")
	write(t, filepath.Join(recipes, "app.yaml"), "source:\n  template: app\nafter:\n  - touch hook-ran\n")
	recipe, err := Get("app")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	target := filepath.Join(t.TempDir(), "myapp")
	var out bytes.Buffer
	if _, err := Create(CreateOptions{
		Recipe: recipe, Target: target, NoRegister: true, NoHooks: true, Out: &out, Log: &out,
	}); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(target, "hook-ran")); !os.IsNotExist(err) {
		t.Error("--no-hooks ran the hook anyway")
	}
}

func TestCreate_AFailingHookIsReported(t *testing.T) {
	recipes, templates := useStores(t)
	write(t, filepath.Join(templates, "app", "go.mod.tmpl"), "module x\n")
	write(t, filepath.Join(recipes, "app.yaml"),
		"source:\n  template: app\nafter:\n  - exit 3\n  - touch never-reached\n")
	recipe, err := Get("app")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	target := filepath.Join(t.TempDir(), "myapp")
	var out bytes.Buffer
	_, err = Create(CreateOptions{Recipe: recipe, Target: target, NoRegister: true, Out: &out, Log: &out})
	if err == nil {
		t.Fatal("Create() error = nil, want the failing hook reported")
	}
	if !strings.Contains(err.Error(), "exit 3") {
		t.Errorf("error = %v, want it to name the command", err)
	}
	// The second command assumed the first one worked.
	if _, statErr := os.Stat(filepath.Join(target, "never-reached")); !os.IsNotExist(statErr) {
		t.Error("the rest of the commands ran after a failure")
	}
	// The files are still there: only the hook failed.
	if _, statErr := os.Stat(filepath.Join(target, "go.mod")); statErr != nil {
		t.Error("the project was rolled back, which is not what was asked")
	}
}

func TestCreate_DryRunListsTheHooks(t *testing.T) {
	recipes, templates := useStores(t)
	write(t, filepath.Join(templates, "app", "go.mod.tmpl"), "module x\n")
	write(t, filepath.Join(recipes, "app.yaml"), "source:\n  template: app\nafter:\n  - touch hook-ran\n")
	recipe, err := Get("app")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	target := filepath.Join(t.TempDir(), "myapp")
	var out bytes.Buffer
	if _, err := Create(CreateOptions{Recipe: recipe, Target: target, DryRun: true, Out: &out, Log: &out}); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if !strings.Contains(out.String(), "[DRY RUN] Would run: touch hook-ran") {
		t.Errorf("out = %q, want the commands listed", out.String())
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Error("the dry run created the project")
	}
}

func TestCreate_SetsUpTheRemote(t *testing.T) {
	recipes, templates := useStores(t)
	write(t, filepath.Join(templates, "app", "go.mod.tmpl"), "module x\n")
	write(t, filepath.Join(recipes, "app.yaml"), `
source:
  template: app
register:
  remote:
    host: example
    group: me
`)

	lazypath.SetTestConfig(lazypath.Config{GitServers: []lazypath.GitServer{
		{Name: "example", HTTPS: "https://git.example.com", SSH: "git@git.example.com"},
	}})
	t.Cleanup(lazypath.ResetTestConfig)

	recipe, err := Get("app")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	target := filepath.Join(t.TempDir(), "myapp")
	var out bytes.Buffer
	if _, err := Create(CreateOptions{Recipe: recipe, Target: target, NoRegister: true, Out: &out, Log: &out}); err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	// It is a repository now, pointed somewhere.
	cmd := exec.Command("git", "-C", target, "remote", "get-url", "origin")
	url, err := cmd.Output()
	if err != nil {
		t.Fatalf("no origin: %v", err)
	}
	if !strings.Contains(string(url), "me/myapp") {
		t.Errorf("origin = %q, want it built from the configured server", url)
	}
	// The repository is not created on the server, and nothing pretends it is.
	if strings.Contains(out.String(), "created") {
		t.Errorf("out = %q, want no claim that the remote repository exists", out.String())
	}
}
