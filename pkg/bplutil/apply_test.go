package bplutil

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"gitlab.com/dynamo-tools/projekt/pkg/tplutil"
)

// recipeProject creates a project from a recipe and returns where it is.
func recipeProject(t *testing.T, after []string) (project string, store string) {
	t.Helper()
	recipes, store := useStores(t)
	write(t, filepath.Join(store, "app", "main.txt.tmpl"), "hello {{ .Values.who }}\n")

	recipe := "description: an app\nsource:\n  template: app\nvars:\n  - name: who\n    default: world\nregister:\n  skip: true\n"
	for _, command := range after {
		recipe += "after:\n  - '" + command + "'\n"
	}
	write(t, filepath.Join(recipes, "app.yaml"), recipe)

	project = filepath.Join(t.TempDir(), "myapp")
	found, err := Get("app")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if _, err := Create(CreateOptions{
		Recipe: found, Target: project, Values: tplutil.Values{"who": "there"},
		NoRegister: true, Log: &bytes.Buffer{},
	}); err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	return project, store
}

func TestCreate_RecordsTheRecipe(t *testing.T) {
	project, _ := recipeProject(t, nil)

	record, err := tplutil.LoadRecord(project)
	if err != nil {
		t.Fatalf("LoadRecord() error = %v", err)
	}
	entry, ok := record.FindRecipe("app")
	if !ok {
		t.Fatalf("LoadRecord() = %+v, want the recipe remembered", record)
	}
	if entry.Template != "app" || entry.Name != "myapp" {
		t.Errorf("recipe record = %+v, want the template and the name it ran with", entry)
	}
	if entry.Values["who"] != "there" {
		t.Errorf("recipe values = %v, want the answers it ran with", entry.Values)
	}
}

func TestApply_ReplaysTheRecordedAnswers(t *testing.T) {
	project, store := recipeProject(t, nil)
	write(t, filepath.Join(store, "app", "main.txt.tmpl"), "hello {{ .Values.who }}, again\n")

	// Nothing is passed: the recipe's own answers come back from the project.
	applied, err := Apply(ApplyOptions{Project: project, Log: &bytes.Buffer{}})
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if len(applied) != 1 || applied[0].Recipe.Name != "app" {
		t.Fatalf("Apply() = %+v, want the recorded recipe", applied)
	}
	if got := readFile(t, filepath.Join(project, "main.txt")); got != "hello there, again\n" {
		t.Errorf("main.txt = %q, want the recorded answer replayed", got)
	}
}

func TestApply_DoesNotRunTheRecipeCommandsAgainByDefault(t *testing.T) {
	project, _ := recipeProject(t, []string{"echo ran >> hook.txt"})
	created := readFile(t, filepath.Join(project, "hook.txt"))

	if _, err := Apply(ApplyOptions{Project: project, Log: &bytes.Buffer{}}); err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if got := readFile(t, filepath.Join(project, "hook.txt")); got != created {
		t.Errorf("hook.txt = %q, want the creation command left alone", got)
	}

	var log bytes.Buffer
	if _, err := Apply(ApplyOptions{Project: project, Hooks: true, Log: &log, HookOut: &log, HookErr: &log}); err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if got := readFile(t, filepath.Join(project, "hook.txt")); got == created {
		t.Errorf("hook.txt = %q, want --hooks to run it again", got)
	}
}

func TestApply_SaysSoWhenTheProjectRecordsNoRecipe(t *testing.T) {
	useStores(t)

	_, err := Apply(ApplyOptions{Project: t.TempDir(), Log: &bytes.Buffer{}})
	if err == nil || !strings.Contains(err.Error(), "t apply") {
		t.Errorf("Apply() error = %v, want it to point at `t apply`", err)
	}
}

func TestApply_RefusesARecipeThatClonesARepository(t *testing.T) {
	recipes, _ := useStores(t)
	write(t, filepath.Join(recipes, "cloned.yaml"), "description: x\nsource:\n  repo: github:me/starter\n")

	recipe, err := Get("cloned")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	_, err = Apply(ApplyOptions{Recipes: []Recipe{recipe}, Project: t.TempDir(), Log: &bytes.Buffer{}})
	if err == nil || !strings.Contains(err.Error(), "cannot be applied again") {
		t.Errorf("Apply() error = %v, want a clone to be refused", err)
	}
}
