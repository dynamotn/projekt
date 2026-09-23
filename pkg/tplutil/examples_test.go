package tplutil

import (
	"os"
	"path/filepath"
	"testing"
)

// exampleStore is the starter set shipped with the repository.
const exampleStore = "../../examples/templates"

// TestExampleTemplatesRender renders every example template with no values at
// all. A template whose defaults do not render is broken for the person trying
// the tool for the first time, which is exactly who reaches for these.
func TestExampleTemplatesRender(t *testing.T) {
	if _, err := os.Stat(exampleStore); err != nil {
		t.Skipf("no example store at %s: %v", exampleStore, err)
	}

	previous := TemplateDir
	TemplateDir = exampleStore
	t.Setenv("PROJEKT_TEMPLATE_DIR", "")
	t.Cleanup(func() { TemplateDir = previous })

	templates, err := List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(templates) == 0 {
		t.Fatalf("List() found no example template in %s", exampleStore)
	}

	for _, tpl := range templates {
		t.Run(tpl.Name, func(t *testing.T) {
			dest := filepath.Join(t.TempDir(), "out")
			if !tpl.IsDir() {
				dest = filepath.Join(dest, tpl.Name)
			}

			written, err := Render(RenderOptions{
				Template: tpl,
				Dest:     dest,
				Name:     "example",
			})
			if err != nil {
				t.Fatalf("Render(%s) error = %v", tpl.Name, err)
			}
			if len(written) == 0 {
				t.Fatalf("Render(%s) wrote nothing", tpl.Name)
			}
			for _, path := range written {
				info, err := os.Stat(path)
				if err != nil {
					t.Fatalf("Render(%s) reported %s, which is not there: %v", tpl.Name, path, err)
				}
				if info.Size() == 0 {
					t.Errorf("Render(%s) wrote an empty %s", tpl.Name, path)
				}
			}
		})
	}
}

// TestExampleValuesRender renders the templates that ship with a values file,
// using it, so that a values file and its template cannot drift apart.
func TestExampleValuesRender(t *testing.T) {
	if _, err := os.Stat(exampleStore); err != nil {
		t.Skipf("no example store at %s: %v", exampleStore, err)
	}

	previous := TemplateDir
	TemplateDir = exampleStore
	t.Setenv("PROJEKT_TEMPLATE_DIR", "")
	t.Cleanup(func() { TemplateDir = previous })

	cases := map[string]string{
		"go-cli":             "go-cli.yaml",
		"docker-compose.yml": "docker-compose.yaml",
		"terraform-module":   "terraform-module.yaml",
		"invoice":            "invoice.yaml",
		"monthly-budget":     "monthly-budget.yaml",
		"threat-model":       "threat-model.yaml",
		"incident-report":    "incident-report.yaml",
	}

	for name, valuesFile := range cases {
		t.Run(name, func(t *testing.T) {
			tpl, err := Get(name)
			if err != nil {
				t.Fatalf("Get(%s) error = %v", name, err)
			}
			values, err := LoadValuesFiles([]string{filepath.Join("../../examples/values", valuesFile)})
			if err != nil {
				t.Fatalf("LoadValuesFiles(%s) error = %v", valuesFile, err)
			}

			dest := filepath.Join(t.TempDir(), "out")
			if !tpl.IsDir() {
				dest = filepath.Join(dest, name)
			}
			if _, err := Render(RenderOptions{
				Template: tpl,
				Dest:     dest,
				Name:     "example",
				Values:   values,
			}); err != nil {
				t.Fatalf("Render(%s) with %s error = %v", name, valuesFile, err)
			}
		})
	}
}
