package hookutil

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/viper"

	"gitlab.com/dynamo-tools/projekt/pkg/lazypath"
	"gitlab.com/dynamo-tools/projekt/pkg/tplutil"
)

// hooked sets up a project with hooks of its own, a workspace with hooks, a
// tag rule, and a plain project, in a configuration file the package reads.
func hooked(t *testing.T) (root string) {
	t.Helper()
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv(NoHooksEnv, "")

	root = t.TempDir()
	for _, path := range []string{"api", "oss/tool", "plain"} {
		if err := os.MkdirAll(filepath.Join(root, path), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	config := filepath.Join(root, "config.yaml")
	content := `
folders:
  - path: ` + filepath.Join(root, "api") + `
    tags: [k8s]
    hooks:
      enter: ["export KUBECONFIG=~/.kube/api", "echo in api"]
      leave: ["unset KUBECONFIG"]
  - path: ` + filepath.Join(root, "oss") + `
    prefix: oss
    is_workspace: true
    hooks:
      enter: ["echo oss"]
  - path: ` + filepath.Join(root, "plain") + `
hooks:
  - tags: [k8s]
    enter: ["kubectl config use-context api"]
    leave: ["kubectl config use-context default"]
`
	if err := os.WriteFile(config, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	load(t, config)
	return root
}

func load(t *testing.T, config string) {
	t.Helper()
	previous := lazypath.CfgFile
	lazypath.CfgFile = config
	lazypath.ResetTestConfig()
	viper.Reset()
	lazypath.InitConfig()
	t.Cleanup(func() {
		lazypath.CfgFile = previous
		lazypath.ResetTestConfig()
		viper.Reset()
	})
}

func transition(t *testing.T, from, to string) (string, error) {
	t.Helper()
	var out bytes.Buffer
	err := Transition(&out, TransitionOptions{From: from, To: to})
	return out.String(), err
}

func TestTransition_EnterRunsFolderThenTagHooks(t *testing.T) {
	root := hooked(t)

	got, err := transition(t, filepath.Join(root, "plain"), filepath.Join(root, "api"))
	if err != nil {
		t.Fatalf("Transition() error = %v", err)
	}
	want := "export KUBECONFIG=~/.kube/api\necho in api\nkubectl config use-context api\n"
	if got != want {
		t.Errorf("Transition() =\n%q\nwant\n%q", got, want)
	}
}

func TestTransition_LeaveComesBeforeEnter(t *testing.T) {
	root := hooked(t)

	got, err := transition(t, filepath.Join(root, "api", "cmd"), filepath.Join(root, "oss", "tool"))
	if err != nil {
		t.Fatalf("Transition() error = %v", err)
	}
	want := "unset KUBECONFIG\nkubectl config use-context default\necho oss\n"
	if got != want {
		t.Errorf("Transition() =\n%q\nwant\n%q", got, want)
	}
}

func TestTransition_SameProjectRunsNothing(t *testing.T) {
	root := hooked(t)

	got, err := transition(t, filepath.Join(root, "api"), filepath.Join(root, "api", "internal"))
	if err != nil || got != "" {
		t.Errorf("Transition() = %q, %v; want nothing within one project", got, err)
	}
}

func TestTransition_OutsideAnyProject(t *testing.T) {
	root := hooked(t)

	got, err := transition(t, t.TempDir(), filepath.Join(root, "plain"))
	if err != nil || got != "" {
		t.Errorf("Transition() = %q, %v; want nothing for a project without hooks", got, err)
	}
	got, err = transition(t, filepath.Join(root, "api"), t.TempDir())
	if err != nil || got != "unset KUBECONFIG\nkubectl config use-context default\n" {
		t.Errorf("Transition() = %q, %v; want only the leave hooks", got, err)
	}
}

func TestTransition_DisabledByEnv(t *testing.T) {
	root := hooked(t)
	t.Setenv(NoHooksEnv, "1")

	got, err := transition(t, "", filepath.Join(root, "api"))
	if err != nil || got != "" {
		t.Errorf("Transition() = %q, %v; want nothing with %s set", got, err, NoHooksEnv)
	}
}

func TestShow(t *testing.T) {
	hooked(t)

	var out bytes.Buffer
	if err := Show(&out, "api"); err != nil {
		t.Fatalf("Show() error = %v", err)
	}
	for _, want := range []string{"enter:", "leave:", "export KUBECONFIG=~/.kube/api\t# folder ", "kubectl config use-context api\t# tags k8s"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("Show() = %q, want it to contain %q", out.String(), want)
		}
	}

	out.Reset()
	if err := Show(&out, "plain"); err != nil || !strings.Contains(out.String(), "no hooks") {
		t.Errorf("Show(plain) = %q, %v", out.String(), err)
	}
}

// includedFromRepo writes an included configuration inside a repository with
// a remote, which is what makes its hooks somebody else's until trusted.
func includedFromRepo(t *testing.T) (root, include string) {
	t.Helper()
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	t.Setenv(NoHooksEnv, "")

	root = t.TempDir()
	team := filepath.Join(root, "team")
	if err := os.MkdirAll(filepath.Join(root, "svc"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(team, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"init", "-q"}, {"remote", "add", "origin", "https://example.com/team.git"}} {
		cmd := exec.Command("git", append([]string{"-C", team}, args...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}

	include = filepath.Join(team, "team.yaml")
	if err := os.WriteFile(include, []byte(`
folders:
  - path: `+filepath.Join(root, "svc")+`
    hooks:
      enter: ["echo from team"]
`), 0o644); err != nil {
		t.Fatal(err)
	}
	config := filepath.Join(root, "config.yaml")
	if err := os.WriteFile(config, []byte("include:\n  - "+include+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	load(t, config)
	return root, include
}

func TestTransition_IncludedFileNeedsTrust(t *testing.T) {
	root, include := includedFromRepo(t)

	previous := tplutil.AskTrust
	tplutil.AskTrust = func(tplutil.Origin, string) (bool, error) { return false, nil }
	t.Cleanup(func() { tplutil.AskTrust = previous })

	got, err := transition(t, "", filepath.Join(root, "svc"))
	if err == nil {
		t.Fatal("Transition() error = nil, want a refusal for an untrusted include")
	}
	if !strings.Contains(err.Error(), "projekt config trust "+include) {
		t.Errorf("error = %v, want the command that trusts the file", err)
	}
	if got != "" {
		t.Errorf("Transition() printed %q, want nothing untrusted", got)
	}

	if err := TrustSource(include); err != nil {
		t.Fatalf("TrustSource() error = %v", err)
	}
	got, err = transition(t, "", filepath.Join(root, "svc"))
	if err != nil || got != "echo from team\n" {
		t.Errorf("after trust: Transition() = %q, %v", got, err)
	}
}
