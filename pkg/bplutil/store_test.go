package bplutil

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gitlab.com/dynamo.foss/projekt/pkg/lazypath"
	"gitlab.com/dynamo.foss/projekt/pkg/tplutil"
)

// useStores points the package at a temporary boilerplate store and a
// temporary template store, and leaves the configuration empty.
func useStores(t *testing.T) (recipes string, templates string) {
	t.Helper()
	recipes = t.TempDir()
	templates = t.TempDir()

	previousRecipes, previousTemplates := RecipeDir, tplutil.TemplateDir
	RecipeDir, tplutil.TemplateDir = recipes, templates
	t.Setenv("PROJEKT_BOILERPLATE_DIR", "")
	t.Setenv("PROJEKT_TEMPLATE_DIR", "")
	lazypath.SetTestConfig(lazypath.Config{})
	t.Cleanup(func() {
		RecipeDir, tplutil.TemplateDir = previousRecipes, previousTemplates
		lazypath.ResetTestConfig()
	})
	return recipes, templates
}

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
}

// writeGoCLI writes a small folder template and a recipe that creates from it.
func writeGoCLI(t *testing.T, recipes, templates, recipe string) {
	t.Helper()
	write(t, filepath.Join(templates, "app", "go.mod.tmpl"), "module {{ .Values.module }}\n")
	write(t, filepath.Join(templates, "app", "cmd", "{{ .Name }}", "main.go.tmpl"), "package main // {{ .Name }}\n")
	write(t, filepath.Join(recipes, "app.yaml"), recipe)
}

func TestDir_Default(t *testing.T) {
	previous := RecipeDir
	RecipeDir = ""
	t.Setenv("PROJEKT_BOILERPLATE_DIR", "")
	t.Cleanup(func() { RecipeDir = previous })

	dir, err := Dir()
	if err != nil {
		t.Fatalf("Dir() error = %v", err)
	}
	if dir != DefaultRecipeDir() {
		t.Errorf("Dir() = %v, want %v", dir, DefaultRecipeDir())
	}
}

func TestDir_FlagWinsOverEnv(t *testing.T) {
	previous := RecipeDir
	RecipeDir = "/tmp/from-flag"
	t.Setenv("PROJEKT_BOILERPLATE_DIR", "/tmp/from-env")
	t.Cleanup(func() { RecipeDir = previous })

	dir, err := Dir()
	if err != nil {
		t.Fatalf("Dir() error = %v", err)
	}
	if dir != "/tmp/from-flag" {
		t.Errorf("Dir() = %v, want /tmp/from-flag", dir)
	}
}

func TestList_MissingStoreIsNotAnError(t *testing.T) {
	previous := RecipeDir
	RecipeDir = filepath.Join(t.TempDir(), "nothing-here")
	t.Setenv("PROJEKT_BOILERPLATE_DIR", "")
	t.Cleanup(func() { RecipeDir = previous })

	recipes, err := List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(recipes) != 0 {
		t.Errorf("List() = %v, want nothing", recipes)
	}
}

func TestList_SkipsWhatIsNotARecipe(t *testing.T) {
	recipes, templates := useStores(t)
	writeGoCLI(t, recipes, templates, "source:\n  template: app\n")
	write(t, filepath.Join(recipes, "notes.txt"), "not a recipe")
	write(t, filepath.Join(recipes, ".hidden.yaml"), "source:\n  template: app\n")
	// A broken recipe is skipped with a warning rather than hiding the rest.
	write(t, filepath.Join(recipes, "broken.yaml"), "source: {}\n")

	list, err := List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(list) != 1 || list[0].Name != "app" {
		t.Errorf("List() = %v, want only app", list)
	}
}

func TestGet(t *testing.T) {
	recipes, templates := useStores(t)
	writeGoCLI(t, recipes, templates, `
description: An app
source:
  template: app
register:
  prefix: work
  tags: [go]
`)

	recipe, err := Get("app")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if recipe.Description != "An app" || recipe.Source.Template != "app" {
		t.Errorf("Get() = %#v", recipe)
	}
	if recipe.Register.Prefix != "work" || len(recipe.Register.Tags) != 1 {
		t.Errorf("Get().Register = %#v", recipe.Register)
	}
	if recipe.Origin() != "template:app" {
		t.Errorf("Origin() = %v", recipe.Origin())
	}
}

func TestGet_Errors(t *testing.T) {
	recipes, templates := useStores(t)
	writeGoCLI(t, recipes, templates, "source:\n  template: app\n")

	for _, name := range []string{"", "unknown", "../app", "sub/app", ".", ".."} {
		if _, err := Get(name); err == nil {
			t.Errorf("Get(%q) error = nil, want an error", name)
		}
	}
}

func TestValidate(t *testing.T) {
	cases := map[string]string{
		"nothing to create from": "description: x\n",
		"more than one origin":   "source:\n  template: app\n  repo: github:me/x\n",
		"repo is not supported":  "source:\n  repo: github:me/x\n",
		"command not supported":  "source:\n  command: [cargo, new]\n",
		"nameless variable":      "source:\n  template: app\nvars:\n  - prompt: hi\n",
		"choice with no choices": "source:\n  template: app\nvars:\n  - name: pick\n    type: choice\n",
	}

	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			recipes, _ := useStores(t)
			write(t, filepath.Join(recipes, "bad.yaml"), body)

			if _, err := Get("bad"); err == nil {
				t.Errorf("Get() error = nil, want an error for %s", name)
			}
		})
	}
}

func TestValidate_UnsupportedOriginSaysSo(t *testing.T) {
	recipes, _ := useStores(t)
	write(t, filepath.Join(recipes, "later.yaml"), "source:\n  repo: github:me/starter\n")

	_, err := Get("later")
	if err == nil {
		t.Fatal("Get() error = nil, want an error")
	}
	// The message has to say what to do instead, not just refuse.
	if !strings.Contains(err.Error(), "source.template") {
		t.Errorf("Get() error = %v, want it to point at source.template", err)
	}
}
