package tplutil

import (
	"go.yaml.in/yaml/v3"
	"io"
	"os"
	"path/filepath"
	"strings"
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
		"agent-docs":         "agent-docs.yaml",
		"agent-skill":        "agent-skill.yaml",
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

// TestExampleTemplatesAsk checks that every example template can say what it
// needs: a manifest that parses, or keys read out of the template itself.
func TestExampleTemplatesAsk(t *testing.T) {
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

	for _, tpl := range templates {
		t.Run(tpl.Name, func(t *testing.T) {
			vars, err := Vars(tpl)
			if err != nil {
				t.Fatalf("Vars(%s) error = %v", tpl.Name, err)
			}

			base, err := BaseContext(RenderOptions{Template: tpl, Name: "example"})
			if err != nil {
				t.Fatalf("BaseContext(%s) error = %v", tpl.Name, err)
			}

			seen := map[string]bool{}
			for _, v := range vars {
				if seen[v.Name] {
					t.Errorf("%s asks for %q twice", tpl.Name, v.Name)
				}
				seen[v.Name] = true

				// Every default must render, or the first question fails.
				if _, err := (Prompter{In: strings.NewReader(""), Out: io.Discard}).
					Ask([]Var{v}, Values{}, base); err != nil && !v.Required {
					t.Errorf("%s: asking for %q failed: %v", tpl.Name, v.Name, err)
				}
			}
		})
	}
}

// renderExample renders one example template into dest.
func renderExample(t *testing.T, name, dest, projectName string, force bool, values ...Values) ([]string, error) {
	t.Helper()
	previous := TemplateDir
	TemplateDir = exampleStore
	t.Setenv("PROJEKT_TEMPLATE_DIR", "")
	t.Cleanup(func() { TemplateDir = previous })

	tpl, err := Get(name)
	if err != nil {
		t.Fatalf("Get(%s) error = %v", name, err)
	}
	o := RenderOptions{Template: tpl, Dest: dest, Name: projectName, Force: force}
	if len(values) > 0 {
		o.Values = values[0]
	}
	return Render(o)
}

// TestExample_AgentDocsLinksEveryAgentToOneFile: CLAUDE.md and GEMINI.md are
// links, so there is one file to keep up to date.
func TestExample_AgentDocsLinksEveryAgentToOneFile(t *testing.T) {
	dest := filepath.Join(t.TempDir(), "proj")
	if _, err := renderExample(t, "agent-docs", dest, "", false); err != nil {
		t.Fatalf("Render error = %v", err)
	}

	if info, err := os.Lstat(filepath.Join(dest, "AGENTS.md")); err != nil || !info.Mode().IsRegular() {
		t.Fatalf("AGENTS.md should be a regular file: %v, %v", info, err)
	}
	agents, _ := os.ReadFile(filepath.Join(dest, "AGENTS.md"))
	for _, name := range []string{"CLAUDE.md", "GEMINI.md"} {
		target, err := os.Readlink(filepath.Join(dest, name))
		if err != nil || target != "AGENTS.md" {
			t.Errorf("%s: Readlink = %q, %v; want a link to AGENTS.md", name, target, err)
		}
		if got, _ := os.ReadFile(filepath.Join(dest, name)); string(got) != string(agents) {
			t.Errorf("%s reads %q, want the content of AGENTS.md", name, got)
		}
	}
}

// TestExample_AgentDocsRendersWithNothing: no values, outside any project, the
// file is a title and nothing else.
func TestExample_AgentDocsRendersWithNothing(t *testing.T) {
	dest := filepath.Join(t.TempDir(), "proj")
	if _, err := renderExample(t, "agent-docs", dest, "", false); err != nil {
		t.Fatalf("Render error = %v", err)
	}

	got, _ := os.ReadFile(filepath.Join(dest, "AGENTS.md"))
	if !strings.HasPrefix(string(got), "# proj\n") {
		t.Errorf("AGENTS.md = %q, want it to start with the folder's name as a title", got)
	}
	for _, unwanted := range []string{"## Commands", "## Conventions", "## Do not touch", "<no value>"} {
		if strings.Contains(string(got), unwanted) {
			t.Errorf("AGENTS.md holds %q with no values:\n%s", unwanted, got)
		}
	}
}

// TestExample_AgentDocsRefusesAnExistingFile: a CLAUDE.md somebody wrote is
// theirs; only Force replaces it.
func TestExample_AgentDocsRefusesAnExistingFile(t *testing.T) {
	dest := t.TempDir()
	mine := filepath.Join(dest, "CLAUDE.md")
	if err := os.WriteFile(mine, []byte("mine"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, err := renderExample(t, "agent-docs", dest, "", false)
	if err == nil || !strings.Contains(err.Error(), "CLAUDE.md") {
		t.Fatalf("Render error = %v, want one naming CLAUDE.md", err)
	}
	if got, _ := os.ReadFile(mine); string(got) != "mine" {
		t.Errorf("CLAUDE.md = %q, want it untouched", got)
	}
	// Not asserted: whether AGENTS.md was written. The engine writes files in
	// order, so it may already be there when CLAUDE.md is refused.

	if _, err := renderExample(t, "agent-docs", dest, "", true); err != nil {
		t.Fatalf("Render with Force error = %v", err)
	}
	if target, err := os.Readlink(mine); err != nil || target != "AGENTS.md" {
		t.Errorf("CLAUDE.md after Force: Readlink = %q, %v; want a link to AGENTS.md", target, err)
	}
}

// skillFrontMatter renders agent-skill and parses the YAML between its `---`
// lines, which is what an agent does before it trusts the skill.
func skillFrontMatter(t *testing.T, values Values) map[string]string {
	t.Helper()
	dest := filepath.Join(t.TempDir(), "release-notes")
	if _, err := renderExample(t, "agent-skill", dest, "release-notes", false, values); err != nil {
		t.Fatalf("Render error = %v", err)
	}
	got, _ := os.ReadFile(filepath.Join(dest, "SKILL.md"))
	rest, ok := strings.CutPrefix(string(got), "---\n")
	front, _, found := strings.Cut(rest, "\n---\n")
	if !ok || !found {
		t.Fatalf("SKILL.md has no front matter:\n%s", got)
	}
	var meta map[string]string
	if err := yaml.Unmarshal([]byte(front), &meta); err != nil {
		t.Fatalf("front matter is not valid YAML: %v\n%s", err, front)
	}
	return meta
}

// TestExample_AgentSkillFrontMatter: the description is what an agent matches
// a request against, so it must be there, and it must survive YAML.
func TestExample_AgentSkillFrontMatter(t *testing.T) {
	meta := skillFrontMatter(t, nil)
	if meta["name"] != "release-notes" || !strings.HasPrefix(meta["description"], "Use when") {
		t.Errorf("default front matter = %v, want the name and a \"Use when\" trigger", meta)
	}

	const given = `Use when asked: "release notes" [draft]`
	meta = skillFrontMatter(t, Values{"description": given})
	if meta["name"] != "release-notes" || meta["description"] != given {
		t.Errorf("front matter = %v, want name release-notes and description %q", meta, given)
	}
}
