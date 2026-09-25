package root

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gitlab.com/dynamo-tools/projekt/pkg/bplutil"
	"gitlab.com/dynamo-tools/projekt/pkg/lazypath"
	"gitlab.com/dynamo-tools/projekt/pkg/tplutil"
)

// stores writes a boilerplate store and a template store for one test.
func stores(t *testing.T, recipe string) (recipeDir string) {
	t.Helper()
	recipeDir = t.TempDir()
	templateDir := t.TempDir()

	files := map[string]string{
		filepath.Join(templateDir, "app", "go.mod.tmpl"): "module {{ .Values.module }}\n",
		filepath.Join(recipeDir, "app.yaml"):             recipe,
	}
	for path, content := range files {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("MkdirAll() error = %v", err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("WriteFile() error = %v", err)
		}
	}

	previousRecipes, previousTemplates := bplutil.RecipeDir, tplutil.TemplateDir
	bplutil.RecipeDir, tplutil.TemplateDir = recipeDir, templateDir
	t.Setenv("PROJEKT_BOILERPLATE_DIR", "")
	t.Setenv("PROJEKT_TEMPLATE_DIR", "")
	lazypath.SetTestConfig(lazypath.Config{})
	t.Cleanup(func() {
		bplutil.RecipeDir, tplutil.TemplateDir = previousRecipes, previousTemplates
		lazypath.ResetTestConfig()
	})
	return recipeDir
}

func runNew(t *testing.T, in string, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	var out, errOut bytes.Buffer

	cmd := NewBoilerplateNewCmd(&out)
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetIn(strings.NewReader(in))
	cmd.SetArgs(args)

	err = cmd.Execute()
	return out.String(), errOut.String(), err
}

func TestNewCmd_CreatesAndReportsThePlan(t *testing.T) {
	stores(t, "source:\n  template: app\n")
	target := filepath.Join(t.TempDir(), "myapp")

	stdout, _, err := runNew(t, "", "app", target, "--no-register", "--set", "module=example.com/myapp")
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	data, err := os.ReadFile(filepath.Join(target, "go.mod"))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	if string(data) != "module example.com/myapp\n" {
		t.Errorf("go.mod = %q", data)
	}
	for _, want := range []string{"Created", "go.mod", "Not registered"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("stdout = %q, want it to contain %q", stdout, want)
		}
	}
}

func TestNewCmd_DryRunWritesNothing(t *testing.T) {
	stores(t, "source:\n  template: app\n")
	target := filepath.Join(t.TempDir(), "myapp")

	stdout, _, err := runNew(t, "", "app", target, "--dry-run")
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !strings.Contains(stdout, "Would create") {
		t.Errorf("stdout = %q, want the plan", stdout)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Error("--dry-run created the folder")
	}
}

func TestNewCmd_Interactive(t *testing.T) {
	stores(t, `
source:
  template: app
vars:
  - name: module
    prompt: Go module path
    default: "example.com/{{ .Name }}"
`)
	target := filepath.Join(t.TempDir(), "myapp")

	stdout, stderr, err := runNew(t, "\n", "app", target, "--no-register", "-i")
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	data, err := os.ReadFile(filepath.Join(target, "go.mod"))
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	// The empty answer takes the default, which sees the project name.
	if string(data) != "module example.com/myapp\n" {
		t.Errorf("go.mod = %q, want the rendered default", data)
	}
	if !strings.Contains(stderr, "Go module path") {
		t.Errorf("stderr = %q, want the question", stderr)
	}
	if strings.Contains(stdout, "Go module path") {
		t.Errorf("stdout = %q, want no question on stdout", stdout)
	}
}

func TestNewCmd_UnknownRecipe(t *testing.T) {
	stores(t, "source:\n  template: app\n")

	if _, _, err := runNew(t, "", "nope", filepath.Join(t.TempDir(), "x")); err == nil {
		t.Error("Execute() error = nil, want an error for an unknown boilerplate")
	}
}

func TestNewCmd_RefusesANonEmptyFolder(t *testing.T) {
	stores(t, "source:\n  template: app\n")
	target := t.TempDir()
	if err := os.WriteFile(filepath.Join(target, "here"), []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if _, _, err := runNew(t, "", "app", target, "--no-register"); err == nil {
		t.Error("Execute() error = nil, want a refusal on a folder that is not empty")
	}
}

func TestListCmd(t *testing.T) {
	stores(t, "description: An app\nsource:\n  template: app\n")

	var out bytes.Buffer
	cmd := NewBoilerplateListCmd(&out)
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--no-color"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	for _, want := range []string{"NAME", "app", "An app", "template:app"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("list = %q, want it to contain %q", out.String(), want)
		}
	}
}

func TestShowAndPathCmd(t *testing.T) {
	recipeDir := stores(t, "description: An app\nsource:\n  template: app\n")

	var out bytes.Buffer
	show := NewBoilerplateShowCmd(&out)
	show.SetOut(&out)
	show.SetArgs([]string{"app"})
	if err := show.Execute(); err != nil {
		t.Fatalf("show error = %v", err)
	}
	if !strings.Contains(out.String(), "template: app") {
		t.Errorf("show = %q, want the recipe source", out.String())
	}

	out.Reset()
	path := NewBoilerplatePathCmd(&out)
	path.SetOut(&out)
	path.SetArgs([]string{"app"})
	if err := path.Execute(); err != nil {
		t.Fatalf("path error = %v", err)
	}
	if strings.TrimSpace(out.String()) != filepath.Join(recipeDir, "app.yaml") {
		t.Errorf("path = %q", out.String())
	}
}
