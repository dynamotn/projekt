package bplutil

import (
	"path/filepath"
	"strings"
	"testing"

	"gitlab.com/dynamo-tools/projekt/pkg/lazypath"
)

func checkMessages(report Report, severity lazypath.Severity) string {
	var out []string
	for _, diag := range report.Diagnostics {
		if diag.Severity == severity {
			out = append(out, diag.Message)
		}
	}
	return strings.Join(out, "\n")
}

func TestCheck_AGoodRecipeHasNothingToSay(t *testing.T) {
	recipes, templates := useStores(t)
	write(t, filepath.Join(templates, "app", "go.mod.tmpl"), "module {{ .Values.module }}\n")
	write(t, filepath.Join(recipes, "app.yaml"),
		"description: an app\nsource:\n  template: app\nvars:\n  - name: module\n    default: example.com/x\nregister:\n  skip: true\n")

	if report := Check("app"); len(report.Diagnostics) != 0 {
		t.Errorf("Check() = %v, want nothing to report", report.Diagnostics)
	}
}

func TestCheck_ReportsEveryProblemNotJustTheFirst(t *testing.T) {
	recipes, _ := useStores(t)
	write(t, filepath.Join(recipes, "bad.yaml"),
		"description: broken\nsource: {}\nvars:\n  - name: \"\"\n  - name: pick\n    type: choice\n")

	report := Check("bad")
	got := checkMessages(report, lazypath.SeverityError)
	if !strings.Contains(got, "nothing to create from") {
		t.Errorf("Check() = %q, want the missing source reported", got)
	}
	if !strings.Contains(got, "no name") || !strings.Contains(got, "no choices") {
		t.Errorf("Check() = %q, want both variable problems reported", got)
	}
}

func TestCheck_ReportsAMissingTemplate(t *testing.T) {
	recipes, _ := useStores(t)
	write(t, filepath.Join(recipes, "app.yaml"), "description: x\nsource:\n  template: nowhere\n")

	report := Check("app")
	if report.Errors() == 0 || !strings.Contains(checkMessages(report, lazypath.SeverityError), "source.template") {
		t.Errorf("Check() = %v, want the missing template reported", report.Diagnostics)
	}
}

func TestCheck_ReportsATemplateThatDoesNotCheckOut(t *testing.T) {
	recipes, templates := useStores(t)
	write(t, filepath.Join(templates, "app", "main.txt.tmpl"), "{{ .Name \n")
	write(t, filepath.Join(recipes, "app.yaml"), "description: x\nsource:\n  template: app\n")

	report := Check("app")
	if !strings.Contains(checkMessages(report, lazypath.SeverityError), "t check app") {
		t.Errorf("Check() = %v, want it to point at the template's own check", report.Diagnostics)
	}
}

func TestCheck_ReportsAnAfterCommandThatDoesNotParse(t *testing.T) {
	recipes, templates := useStores(t)
	write(t, filepath.Join(templates, "app", "main.txt.tmpl"), "x\n")
	write(t, filepath.Join(recipes, "app.yaml"),
		"description: x\nsource:\n  template: app\nafter:\n  - 'echo {{ .Name '\n")

	report := Check("app")
	if report.Errors() == 0 || !strings.Contains(checkMessages(report, lazypath.SeverityError), "cannot parse") {
		t.Errorf("Check() = %v, want the broken command reported", report.Diagnostics)
	}
}

func TestCheck_ReportsAWorkspaceThatIsNotThere(t *testing.T) {
	recipes, templates := useStores(t)
	write(t, filepath.Join(templates, "app", "main.txt.tmpl"), "x\n")
	write(t, filepath.Join(recipes, "app.yaml"),
		"description: x\nsource:\n  template: app\nregister:\n  workspace: /nowhere/at/all\n")

	got := checkMessages(Check("app"), lazypath.SeverityWarning)
	if !strings.Contains(got, "does not exist yet") {
		t.Errorf("Check() = %q, want the missing workspace reported", got)
	}
}

func TestCheck_ReportsAnUnconfiguredRemoteHost(t *testing.T) {
	recipes, templates := useStores(t)
	write(t, filepath.Join(templates, "app", "main.txt.tmpl"), "x\n")
	write(t, filepath.Join(recipes, "app.yaml"),
		"description: x\nsource:\n  template: app\nregister:\n  remote:\n    host: nowhere\n    group: me\n")

	report := Check("app")
	if report.Errors() == 0 {
		t.Errorf("Check() = %v, want the unknown git server reported", report.Diagnostics)
	}
}

func TestCheck_ComparesTheQuestionsWithTheTemplate(t *testing.T) {
	recipes, templates := useStores(t)
	write(t, filepath.Join(templates, "app", ".data.yaml"), "license: MIT\n")
	write(t, filepath.Join(templates, "app", "main.txt.tmpl"),
		"{{ .Values.module }} {{ .Values.license }} {{ .Values.registry }}\n")
	write(t, filepath.Join(recipes, "app.yaml"),
		"description: x\nsource:\n  template: app\nvars:\n  - name: module\n  - name: unused\n")

	got := checkMessages(Check("app"), lazypath.SeverityWarning)
	if !strings.Contains(got, `asks for "unused"`) {
		t.Errorf("Check() = %q, want the unread question reported", got)
	}
	if !strings.Contains(got, "registry") {
		t.Errorf("Check() = %q, want the unasked value reported", got)
	}
	if strings.Contains(got, "license") {
		t.Errorf("Check() = %q, want a value the data file holds left alone", got)
	}
	if strings.Contains(got, `asks for "module"`) {
		t.Errorf("Check() = %q, want the question the template reads left alone", got)
	}
}

func TestCheckAll_CoversTheStoreIncludingWhatWillNotLoad(t *testing.T) {
	recipes, templates := useStores(t)
	write(t, filepath.Join(templates, "app", "main.txt.tmpl"), "x\n")
	write(t, filepath.Join(recipes, "good.yaml"), "description: x\nsource:\n  template: app\n")
	write(t, filepath.Join(recipes, "bad.yaml"), "description: x\nsource: {}\n")

	reports, err := CheckAll()
	if err != nil {
		t.Fatalf("CheckAll() error = %v", err)
	}
	if len(reports) != 2 {
		t.Fatalf("CheckAll() = %d reports, want one per recipe file", len(reports))
	}

	total := 0
	for _, report := range reports {
		total += report.Errors()
	}
	if total != 1 {
		t.Errorf("CheckAll() found %d errors, want the one broken recipe", total)
	}
}
