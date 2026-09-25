package bplutil

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gitlab.com/dynamo-tools/projekt/pkg/lazypath"
	"gitlab.com/dynamo-tools/projekt/pkg/tplutil"
)

const (
	exampleRecipes   = "../../examples/boilerplates"
	exampleTemplates = "../../examples/templates"
)

// useExampleStores points the package at the stores shipped with the
// repository, and leaves the configuration empty so that nothing is written.
func useExampleStores(t *testing.T) {
	t.Helper()
	for _, dir := range []string{exampleRecipes, exampleTemplates} {
		if _, err := os.Stat(dir); err != nil {
			t.Skipf("no example store at %s: %v", dir, err)
		}
	}

	previousRecipes, previousTemplates := RecipeDir, tplutil.TemplateDir
	RecipeDir, tplutil.TemplateDir = exampleRecipes, exampleTemplates
	t.Setenv("PROJEKT_BOILERPLATE_DIR", "")
	t.Setenv("PROJEKT_TEMPLATE_DIR", "")
	lazypath.SetTestConfig(lazypath.Config{})
	t.Cleanup(func() {
		RecipeDir, tplutil.TemplateDir = previousRecipes, previousTemplates
		lazypath.ResetTestConfig()
	})
}

// TestExampleRecipesCreate creates a project from every example recipe, with
// no values at all. A recipe whose defaults do not work is broken for the
// person trying the tool for the first time, which is exactly who reaches for
// these.
func TestExampleRecipesCreate(t *testing.T) {
	useExampleStores(t)

	recipes, err := List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(recipes) == 0 {
		t.Fatalf("List() found no example recipe in %s", exampleRecipes)
	}

	for _, recipe := range recipes {
		t.Run(recipe.Name, func(t *testing.T) {
			if recipe.Source.Repo != "" {
				// Creating from one would clone it, and a test that needs the
				// network is a test that fails on a train.
				t.Skip("creates from a repository")
			}
			if len(recipe.After) > 0 {
				t.Skip("runs commands of its own")
			}
			target := filepath.Join(t.TempDir(), "project")

			result, err := Create(CreateOptions{
				Recipe:     recipe,
				Target:     target,
				NoRegister: true,
			})
			if err != nil {
				t.Fatalf("Create(%s) error = %v", recipe.Name, err)
			}
			if len(result.Files) == 0 {
				t.Fatalf("Create(%s) wrote nothing", recipe.Name)
			}
			for _, file := range result.Files {
				info, err := os.Stat(file)
				if err != nil {
					t.Fatalf("Create(%s) reported %s, which is not there: %v", recipe.Name, file, err)
				}
				if info.Size() == 0 {
					t.Errorf("Create(%s) wrote an empty %s", recipe.Name, file)
				}
			}
		})
	}
}

// TestExampleRecipesAsk checks that every example recipe can say what it needs
// and that each default renders.
func TestExampleRecipesAsk(t *testing.T) {
	useExampleStores(t)

	recipes, err := List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}

	for _, recipe := range recipes {
		t.Run(recipe.Name, func(t *testing.T) {
			plan, err := Resolve(CreateOptions{
				Recipe:     recipe,
				Target:     filepath.Join(t.TempDir(), "project"),
				NoRegister: true,
			})
			if err != nil {
				t.Fatalf("Resolve(%s) error = %v", recipe.Name, err)
			}

			vars, err := plan.Vars()
			if err != nil {
				t.Fatalf("Vars(%s) error = %v", recipe.Name, err)
			}
			if len(vars) == 0 {
				t.Fatalf("%s asks for nothing, which no example should", recipe.Name)
			}

			base, err := plan.BaseContext()
			if err != nil {
				t.Fatalf("BaseContext(%s) error = %v", recipe.Name, err)
			}

			seen := map[string]bool{}
			for _, v := range vars {
				if seen[v.Name] {
					t.Errorf("%s asks for %q twice", recipe.Name, v.Name)
				}
				seen[v.Name] = true

				if v.Required {
					continue
				}
				if _, err := (tplutil.Prompter{In: strings.NewReader(""), Out: io.Discard}).
					Ask([]tplutil.Var{v}, tplutil.Values{}, base); err != nil {
					t.Errorf("%s: asking for %q failed: %v", recipe.Name, v.Name, err)
				}
			}
		})
	}
}

// TestExampleRecipesReferenceKnownTemplates keeps the two example stores in
// step: a recipe pointing at a template that was renamed is a broken example.
func TestExampleRecipesReferenceKnownTemplates(t *testing.T) {
	useExampleStores(t)

	recipes, err := List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	for _, recipe := range recipes {
		if recipe.Source.Template == "" {
			continue
		}
		if _, err := tplutil.Get(recipe.Source.Template); err != nil {
			t.Errorf("%s creates from %q: %v", recipe.Name, recipe.Source.Template, err)
		}
	}
}
