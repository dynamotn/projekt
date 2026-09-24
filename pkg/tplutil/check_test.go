package tplutil

import (
	"path/filepath"
	"strings"
	"testing"

	"gitlab.com/dynamo.foss/projekt/pkg/lazypath"
)

// messages returns the report's text, joined, which is what the assertions
// below are about.
func messages(report Report, severity lazypath.Severity) string {
	var out []string
	for _, diag := range report.Diagnostics {
		if diag.Severity == severity {
			out = append(out, diag.Message)
		}
	}
	return strings.Join(out, "\n")
}

func checkTemplate(t *testing.T, name string) Report {
	t.Helper()
	tpl, err := Get(name)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	return Check(tpl)
}

func TestCheck_AGoodTemplateHasNothingToSay(t *testing.T) {
	store := useStore(t)
	writeTemplate(t, filepath.Join(store, PartialsDir, "header.tmpl"), "# {{ .Name }}")
	writeTemplate(t, filepath.Join(store, "app", ".vars.yaml"), "vars:\n  - name: module\n    default: example.com/x\n")
	writeTemplate(t, filepath.Join(store, "app", ".data.yaml"), "license: MIT\n")
	writeTemplate(t, filepath.Join(store, "app", "go.mod.tmpl"),
		"{{ template \"header\" . }}\nmodule {{ .Values.module }}\n// {{ .Values.license }}\n")

	if report := checkTemplate(t, "app"); len(report.Diagnostics) != 0 {
		t.Errorf("Check() = %v, want nothing to report", report.Diagnostics)
	}
}

func TestCheck_ReportsASyntaxError(t *testing.T) {
	store := useStore(t)
	writeTemplate(t, filepath.Join(store, "app", "main.txt.tmpl"), "{{ .Name \n")

	report := checkTemplate(t, "app")
	if report.Errors() == 0 || !strings.Contains(messages(report, lazypath.SeverityError), "cannot parse") {
		t.Errorf("Check() = %v, want the unparseable file reported", report.Diagnostics)
	}
}

func TestCheck_ReportsAMissingSharedTemplate(t *testing.T) {
	store := useStore(t)
	writeTemplate(t, filepath.Join(store, "app", "main.txt.tmpl"),
		"{{ template \"header\" . }}\n{{ includeTemplate \"footer\" . }}\n")

	report := checkTemplate(t, "app")
	got := messages(report, lazypath.SeverityError)
	if !strings.Contains(got, `"header"`) || !strings.Contains(got, `"footer"`) {
		t.Errorf("Check() = %q, want both missing shared templates reported", got)
	}
}

func TestCheck_ReportsABadManifest(t *testing.T) {
	store := useStore(t)
	writeTemplate(t, filepath.Join(store, "app", ".vars.yaml"), "delims: [\"<%\"]\n")
	writeTemplate(t, filepath.Join(store, "app", "main.txt.tmpl"), "x\n")

	report := checkTemplate(t, "app")
	if report.Errors() == 0 || !strings.Contains(messages(report, lazypath.SeverityError), "exactly two") {
		t.Errorf("Check() = %v, want the manifest reported", report.Diagnostics)
	}
}

func TestCheck_ReportsAPathSegmentThatRendersAway(t *testing.T) {
	store := useStore(t)
	writeTemplate(t, filepath.Join(store, "app", "{{ .Values.missing }}", "main.txt.tmpl"), "x\n")

	report := checkTemplate(t, "app")
	if report.Errors() == 0 || !strings.Contains(messages(report, lazypath.SeverityError), "never set") {
		t.Errorf("Check() = %v, want the unset path segment reported", report.Diagnostics)
	}
}

func TestCheck_ReportsATemplateThatWritesNothing(t *testing.T) {
	store := useStore(t)
	writeTemplate(t, filepath.Join(store, "app", "main.txt.tmpl"), "x\n")
	writeTemplate(t, filepath.Join(store, "app", IgnoreFile), "*\n")

	report := checkTemplate(t, "app")
	if !strings.Contains(messages(report, lazypath.SeverityWarning), "writes nothing") {
		t.Errorf("Check() = %v, want the empty result reported", report.Diagnostics)
	}
}

func TestCheck_ReportsAQuestionNobodyReads(t *testing.T) {
	store := useStore(t)
	writeTemplate(t, filepath.Join(store, "app", ".vars.yaml"), "vars:\n  - name: unused\n  - name: used\n")
	writeTemplate(t, filepath.Join(store, "app", "main.txt.tmpl"), "{{ .Values.used }}\n")

	got := messages(checkTemplate(t, "app"), lazypath.SeverityWarning)
	if !strings.Contains(got, `"unused"`) {
		t.Errorf("Check() = %q, want the unread question reported", got)
	}
	if strings.Contains(got, `"used"`) {
		t.Errorf("Check() = %q, want the read one left alone", got)
	}
}

func TestCheck_ReportsAValueNobodyAsksFor(t *testing.T) {
	store := useStore(t)
	writeTemplate(t, filepath.Join(store, "app", ".vars.yaml"), "vars:\n  - name: module\n")
	writeTemplate(t, filepath.Join(store, "app", "main.txt.tmpl"),
		"{{ .Values.module }} {{ .Values.registry }}\n")

	got := messages(checkTemplate(t, "app"), lazypath.SeverityWarning)
	if !strings.Contains(got, "registry") {
		t.Errorf("Check() = %q, want the undeclared value reported", got)
	}
}

func TestCheck_LeavesOptionalAndStructuredValuesAlone(t *testing.T) {
	store := useStore(t)
	writeTemplate(t, filepath.Join(store, "app", ".vars.yaml"), "vars:\n  - name: module\n")
	writeTemplate(t, filepath.Join(store, "app", "main.txt.tmpl"), strings.Join([]string{
		"{{ .Values.module }}",
		// Optional by design: the template says what to do without them.
		`{{ .Values.note | default "none" }}`,
		"{{ with .Values.summary }}{{ . }}{{ end }}",
		// Structured by design: a list belongs in a --values file.
		"{{ range .Values.items }}{{ . }}{{ end }}",
		"",
	}, "\n"))

	if got := messages(checkTemplate(t, "app"), lazypath.SeverityWarning); got != "" {
		t.Errorf("Check() = %q, want optional and structured values left alone", got)
	}
}

func TestCheck_CountsADataFileAsAnAnswer(t *testing.T) {
	store := useStore(t)
	writeTemplate(t, filepath.Join(store, DataFile), "company: Acme\n")
	writeTemplate(t, filepath.Join(store, "app", ".vars.yaml"), "vars:\n  - name: module\n")
	writeTemplate(t, filepath.Join(store, "app", ".data.yaml"), "license: MIT\n")
	writeTemplate(t, filepath.Join(store, "app", "main.txt.tmpl"),
		"{{ .Values.module }} {{ .Values.license }} {{ .Values.company }}\n")

	if got := messages(checkTemplate(t, "app"), lazypath.SeverityWarning); got != "" {
		t.Errorf("Check() = %q, want a value a data file holds treated as answered", got)
	}
}

func TestCheck_ATemplateThatAsksWhileRenderingIsNotBroken(t *testing.T) {
	store := useStore(t)
	writeTemplate(t, filepath.Join(store, "app", "main.txt.tmpl"), "{{ promptString \"owner\" }}\n")

	if report := checkTemplate(t, "app"); report.Errors() != 0 {
		t.Errorf("Check() = %v, want a question to be a question and not an error", report.Diagnostics)
	}
}

func TestCheckAll_CoversTheStore(t *testing.T) {
	store := useStore(t)
	writeTemplate(t, filepath.Join(store, "good.tmpl"), "{{ .Name }}\n")
	writeTemplate(t, filepath.Join(store, "bad.tmpl"), "{{ .Name \n")

	reports, err := CheckAll()
	if err != nil {
		t.Fatalf("CheckAll() error = %v", err)
	}
	if len(reports) != 2 {
		t.Fatalf("CheckAll() = %d reports, want one per template", len(reports))
	}

	total := 0
	for _, report := range reports {
		total += report.Errors()
	}
	if total != 1 {
		t.Errorf("CheckAll() found %d errors, want the one broken template", total)
	}
}
