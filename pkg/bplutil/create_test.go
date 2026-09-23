package bplutil

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gitlab.com/dynamo.foss/projekt/pkg/lazypath"
	"gitlab.com/dynamo.foss/projekt/pkg/tplutil"
)

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s) error = %v", path, err)
	}
	return string(data)
}

func TestCreate_IntoAWorkspaceByName(t *testing.T) {
	recipes, templates := useStores(t)
	workspace := t.TempDir()
	writeGoCLI(t, recipes, templates, "source:\n  template: app\nregister:\n  prefix: work\n  tags: [go]\n")

	recipe, err := Get("app")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	result, err := Create(CreateOptions{
		Recipe:     recipe,
		Target:     "myapp",
		Workspace:  workspace,
		Values:     tplutil.Values{"module": "example.com/myapp"},
		NoRegister: true,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	want := filepath.Join(workspace, "myapp")
	if result.Path != want {
		t.Errorf("Create().Path = %v, want %v", result.Path, want)
	}
	if result.Name != "myapp" {
		t.Errorf("Create().Name = %v", result.Name)
	}
	if got := readFile(t, filepath.Join(want, "go.mod")); got != "module example.com/myapp\n" {
		t.Errorf("go.mod = %q", got)
	}
	// The project name reaches the path segments as well.
	if got := readFile(t, filepath.Join(want, "cmd", "myapp", "main.go")); !strings.Contains(got, "myapp") {
		t.Errorf("main.go = %q", got)
	}
}

func TestCreate_PathIsUsedAsItIs(t *testing.T) {
	recipes, templates := useStores(t)
	writeGoCLI(t, recipes, templates, "source:\n  template: app\nregister:\n  workspace: /nowhere\n")

	recipe, err := Get("app")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	target := filepath.Join(t.TempDir(), "sub", "here")

	result, err := Create(CreateOptions{
		Recipe:     recipe,
		Target:     target,
		Values:     tplutil.Values{"module": "example.com/here"},
		NoRegister: true,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	// A path wins over the recipe's workspace, which is what makes it an
	// escape hatch rather than a suggestion.
	if result.Path != target {
		t.Errorf("Create().Path = %v, want %v", result.Path, target)
	}
	if result.Name != "here" {
		t.Errorf("Create().Name = %v, want here", result.Name)
	}
}

func TestCreate_DryRunWritesNothing(t *testing.T) {
	recipes, templates := useStores(t)
	workspace := t.TempDir()
	writeGoCLI(t, recipes, templates, "source:\n  template: app\n")

	recipe, err := Get("app")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	result, err := Create(CreateOptions{
		Recipe:    recipe,
		Target:    "myapp",
		Workspace: workspace,
		DryRun:    true,
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if len(result.Files) != 2 {
		t.Errorf("Create().Files = %v, want the two files it would write", result.Files)
	}
	if result.Registered {
		t.Error("Create() with DryRun registered the project")
	}
	if _, err := os.Stat(filepath.Join(workspace, "myapp")); !os.IsNotExist(err) {
		t.Error("Create() with DryRun created the folder")
	}
}

func TestCreate_RefusesANonEmptyFolder(t *testing.T) {
	recipes, templates := useStores(t)
	writeGoCLI(t, recipes, templates, "source:\n  template: app\n")

	recipe, err := Get("app")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	target := t.TempDir()
	write(t, filepath.Join(target, "already-here"), "x")

	if _, err := Create(CreateOptions{Recipe: recipe, Target: target, NoRegister: true}); err == nil {
		t.Fatal("Create() error = nil, want a refusal on a folder that is not empty")
	}

	// --force goes ahead.
	if _, err := Create(CreateOptions{
		Recipe:     recipe,
		Target:     target,
		Values:     tplutil.Values{"module": "x"},
		Force:      true,
		NoRegister: true,
	}); err != nil {
		t.Fatalf("Create() with Force error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(target, "go.mod")); err != nil {
		t.Errorf("Create() with Force did not write: %v", err)
	}
}

func TestResolve_RegistersByDefault(t *testing.T) {
	recipes, templates := useStores(t)
	writeGoCLI(t, recipes, templates, "source:\n  template: app\nregister:\n  prefix: work\n")

	recipe, err := Get("app")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	plan, err := Resolve(CreateOptions{Recipe: recipe, Target: "myapp", Workspace: t.TempDir()})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if !plan.Register {
		t.Errorf("Resolve().Register = false, want true (reason: %q)", plan.Reason)
	}
	if plan.ShortName() != "work-myapp" {
		t.Errorf("ShortName() = %v, want work-myapp", plan.ShortName())
	}
}

func TestResolve_AWorkspaceAlreadyCoversIt(t *testing.T) {
	recipes, templates := useStores(t)
	workspace := t.TempDir()
	writeGoCLI(t, recipes, templates, "source:\n  template: app\nregister:\n  prefix: ignored\n")

	lazypath.SetTestConfig(lazypath.Config{Folders: []lazypath.Folder{{
		Path:        workspace,
		Prefix:      "oss",
		IsWorkspace: true,
	}}})

	recipe, err := Get("app")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	plan, err := Resolve(CreateOptions{Recipe: recipe, Target: "mytool", Workspace: workspace})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	// The workspace already turns the new folder into a project, so a second
	// entry would only be a second name for it.
	if plan.Register {
		t.Error("Resolve().Register = true, want false inside a workspace")
	}
	if plan.CoveredBy != workspace {
		t.Errorf("Resolve().CoveredBy = %v, want %v", plan.CoveredBy, workspace)
	}
	// The name it answers to is the workspace's, not the recipe's.
	if plan.ShortName() != "oss-mytool" {
		t.Errorf("ShortName() = %v, want oss-mytool", plan.ShortName())
	}
}

func TestResolve_WorkspaceRegexIsRespected(t *testing.T) {
	recipes, templates := useStores(t)
	workspace := t.TempDir()
	writeGoCLI(t, recipes, templates, "source:\n  template: app\n")

	// The workspace only takes folders starting with "go-", so a project that
	// does not match is not covered and needs an entry of its own.
	lazypath.SetTestConfig(lazypath.Config{Folders: []lazypath.Folder{{
		Path:        workspace,
		IsWorkspace: true,
		RegexMatch:  "^go-",
	}}})

	recipe, err := Get("app")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	covered, err := Resolve(CreateOptions{Recipe: recipe, Target: "go-tool", Workspace: workspace})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if covered.Register || covered.CoveredBy == "" {
		t.Errorf("go-tool: Register = %v, CoveredBy = %q, want it covered", covered.Register, covered.CoveredBy)
	}

	outside, err := Resolve(CreateOptions{Recipe: recipe, Target: "rust-tool", Workspace: workspace})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if !outside.Register {
		t.Errorf("rust-tool: Register = false, want true (reason: %q)", outside.Reason)
	}
}

func TestResolve_ReasonsForNotRegistering(t *testing.T) {
	recipes, templates := useStores(t)
	workspace := t.TempDir()
	writeGoCLI(t, recipes, templates, "source:\n  template: app\n")

	recipe, err := Get("app")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	plan, err := Resolve(CreateOptions{Recipe: recipe, Target: "myapp", Workspace: workspace, NoRegister: true})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if plan.Register || plan.Reason == "" {
		t.Errorf("--no-register: Register = %v, Reason = %q", plan.Register, plan.Reason)
	}

	skipped := recipe
	skipped.Register.Skip = true
	plan, err = Resolve(CreateOptions{Recipe: skipped, Target: "myapp", Workspace: workspace})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if plan.Register || !strings.Contains(plan.Reason, "register.skip") {
		t.Errorf("register.skip: Register = %v, Reason = %q", plan.Register, plan.Reason)
	}

	// A folder already in the configuration is not added twice.
	lazypath.SetTestConfig(lazypath.Config{Folders: []lazypath.Folder{{Path: filepath.Join(workspace, "myapp")}}})
	plan, err = Resolve(CreateOptions{Recipe: recipe, Target: "myapp", Workspace: workspace})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if plan.Register || !strings.Contains(plan.Reason, "already in the configuration") {
		t.Errorf("already configured: Register = %v, Reason = %q", plan.Register, plan.Reason)
	}
}

func TestCreate_RegistersTheProject(t *testing.T) {
	recipes, templates := useStores(t)
	workspace := t.TempDir()
	writeGoCLI(t, recipes, templates, "source:\n  template: app\nregister:\n  prefix: work\n  tags: [go]\n  priority: 5\n")

	// A real configuration file, because registering writes to it.
	configFile := filepath.Join(t.TempDir(), "config.yaml")
	previousCfg := lazypath.CfgFile
	lazypath.CfgFile = configFile
	lazypath.ResetTestConfig()
	lazypath.InitConfig()
	t.Cleanup(func() {
		lazypath.CfgFile = previousCfg
		lazypath.ResetTestConfig()
	})

	recipe, err := Get("app")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	result, err := Create(CreateOptions{
		Recipe:    recipe,
		Target:    "myapp",
		Workspace: workspace,
		Values:    tplutil.Values{"module": "example.com/myapp"},
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if !result.Registered {
		t.Fatalf("Create().Registered = false, reason %q", result.Reason)
	}

	folders := lazypath.GetConfig().Folders
	if len(folders) != 1 {
		t.Fatalf("config has %d folders, want 1", len(folders))
	}
	added := folders[0]
	if added.Path != filepath.Join(workspace, "myapp") {
		t.Errorf("added.Path = %v", added.Path)
	}
	if added.Prefix != "work" || added.Priority != 5 || len(added.Tags) != 1 || added.Tags[0] != "go" {
		t.Errorf("added = %#v, want the recipe's prefix, priority and tags", added)
	}
	if !strings.Contains(readFile(t, configFile), "myapp") {
		t.Error("the config file does not mention the new project")
	}
}

func TestPlanVars_FallsBackToTheTemplate(t *testing.T) {
	recipes, templates := useStores(t)
	writeGoCLI(t, recipes, templates, "source:\n  template: app\n")

	recipe, err := Get("app")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	plan, err := Resolve(CreateOptions{Recipe: recipe, Target: "myapp", Workspace: t.TempDir()})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	vars, err := plan.Vars()
	if err != nil {
		t.Fatalf("Vars() error = %v", err)
	}
	// The recipe says nothing, so the template's own keys are what to ask for.
	if len(vars) != 1 || vars[0].Name != "module" {
		t.Errorf("Vars() = %#v, want the template's module key", vars)
	}
}

func TestPlanVars_RecipeWins(t *testing.T) {
	recipes, templates := useStores(t)
	writeGoCLI(t, recipes, templates, `
source:
  template: app
vars:
  - name: module
    prompt: Go module path
    required: true
`)

	recipe, err := Get("app")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	plan, err := Resolve(CreateOptions{Recipe: recipe, Target: "myapp", Workspace: t.TempDir()})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	vars, err := plan.Vars()
	if err != nil {
		t.Fatalf("Vars() error = %v", err)
	}
	if len(vars) != 1 || vars[0].Prompt != "Go module path" || !vars[0].Required {
		t.Errorf("Vars() = %#v, want the recipe's own list", vars)
	}
}

func TestPlanBaseContext(t *testing.T) {
	recipes, templates := useStores(t)
	writeGoCLI(t, recipes, templates, "source:\n  template: app\n")

	recipe, err := Get("app")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	plan, err := Resolve(CreateOptions{Recipe: recipe, Target: "myapp", Workspace: t.TempDir()})
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}

	base, err := plan.BaseContext()
	if err != nil {
		t.Fatalf("BaseContext() error = %v", err)
	}
	// A default such as "example.com/{{ .Name }}" has to see the project name.
	if base["Name"] != "myapp" {
		t.Errorf("BaseContext()[Name] = %v, want myapp", base["Name"])
	}
}

func TestResolve_Errors(t *testing.T) {
	recipes, templates := useStores(t)
	writeGoCLI(t, recipes, templates, "source:\n  template: app\n")

	recipe, err := Get("app")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if _, err := Resolve(CreateOptions{Recipe: recipe, Target: ""}); err == nil {
		t.Error("Resolve() with no target error = nil, want an error")
	}

	missing := recipe
	missing.Source.Template = "not-there"
	if _, err := Resolve(CreateOptions{Recipe: missing, Target: "x", Workspace: t.TempDir()}); err == nil {
		t.Error("Resolve() with an unknown template error = nil, want an error")
	}
}
