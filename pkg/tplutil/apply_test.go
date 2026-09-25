package tplutil

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// applyStore is a small folder template whose CI file can be switched off, so
// every status an apply can report is reachable.
func applyStore(t *testing.T) string {
	t.Helper()
	store := useStore(t)
	writeTemplate(t, filepath.Join(store, "app", "main.txt.tmpl"), "hello {{ .Name }}\n")
	writeTemplate(t, filepath.Join(store, "app", "keep.txt.tmpl"), "steady\n")
	writeTemplate(t, filepath.Join(store, "app", "ci", "job.yml.tmpl"), "job: {{ .Name }}\n")
	writeTemplate(t, filepath.Join(store, "app", IgnoreFile), "{{ if not .Values.ci }}ci/{{ end }}\n")
	return store
}

func create(t *testing.T, dest string, values Values) Template {
	t.Helper()
	tpl, err := Get("app")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if _, err := Render(RenderOptions{Template: tpl, Dest: dest, Name: "myapp", Values: values}); err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	return tpl
}

// statuses reduces a plan to a path -> status map, which is what the tests are
// actually about.
func statuses(changes []Change) map[string]ChangeStatus {
	got := map[string]ChangeStatus{}
	for _, change := range changes {
		got[change.Path] = change.Status
	}
	return got
}

func TestRender_WritesTheRecord(t *testing.T) {
	applyStore(t)
	dest := t.TempDir()
	create(t, dest, Values{"ci": true})

	record, err := LoadRecord(dest)
	if err != nil {
		t.Fatalf("LoadRecord() error = %v", err)
	}
	render, ok := record.Find("app")
	if !ok {
		t.Fatalf("LoadRecord() = %+v, want the app render remembered", record)
	}
	if render.Name != "myapp" {
		t.Errorf("Name = %q, want myapp", render.Name)
	}
	if render.Values["ci"] != true {
		t.Errorf("Values = %v, want the values it ran with", render.Values)
	}
	if len(render.Files) != 3 {
		t.Errorf("Files = %v, want the three files it wrote", render.Files)
	}
	if _, err := os.Stat(filepath.Join(dest, RecordDir, RecordFile)); err != nil {
		t.Errorf("Stat(record) error = %v, want it in the project", err)
	}
}

func TestPlanApply_UnchangedWhenNothingMoved(t *testing.T) {
	store := applyStore(t)
	dest := t.TempDir()
	tpl := create(t, dest, Values{"ci": true})
	_ = store

	changes, err := PlanApply(ApplyOptions{Templates: []Template{tpl}, Dest: dest})
	if err != nil {
		t.Fatalf("PlanApply() error = %v", err)
	}
	for _, change := range changes {
		if change.Writes() {
			t.Errorf("%s = %s, want everything unchanged", change.Path, change.Status)
		}
	}
	if got := Summary(changes); got != "3 unchanged" {
		t.Errorf("Summary() = %q, want 3 unchanged", got)
	}
}

func TestPlanApply_TellsAnOutdatedFileFromAnEditedOne(t *testing.T) {
	store := applyStore(t)
	dest := t.TempDir()
	tpl := create(t, dest, Values{"ci": true})

	// The template moved on, and somebody edited another file by hand.
	writeTemplate(t, filepath.Join(store, "app", "main.txt.tmpl"), "hello {{ .Name }}, again\n")
	writeTemplate(t, filepath.Join(store, "app", "extra.txt.tmpl"), "new\n")
	if err := os.WriteFile(filepath.Join(dest, "keep.txt"), []byte("mine\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	changes, err := PlanApply(ApplyOptions{Templates: []Template{tpl}, Dest: dest})
	if err != nil {
		t.Fatalf("PlanApply() error = %v", err)
	}
	got := statuses(changes)
	want := map[string]ChangeStatus{
		"main.txt":   ChangeUpdated,
		"keep.txt":   ChangeConflict,
		"extra.txt":  ChangeAdded,
		"ci/job.yml": ChangeUnchanged,
	}
	for path, status := range want {
		if got[path] != status {
			t.Errorf("%s = %s, want %s", path, got[path], status)
		}
	}
}

func TestApply_KeepsAConflictAndWritesTheRest(t *testing.T) {
	store := applyStore(t)
	dest := t.TempDir()
	tpl := create(t, dest, Values{"ci": true})

	writeTemplate(t, filepath.Join(store, "app", "main.txt.tmpl"), "hello {{ .Name }}, again\n")
	if err := os.WriteFile(filepath.Join(dest, "keep.txt"), []byte("mine\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if _, err := Apply(ApplyOptions{Templates: []Template{tpl}, Dest: dest}); err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if got := readFile(t, filepath.Join(dest, "main.txt")); got != "hello myapp, again\n" {
		t.Errorf("main.txt = %q, want the template's new content", got)
	}
	if got := readFile(t, filepath.Join(dest, "keep.txt")); got != "mine\n" {
		t.Errorf("keep.txt = %q, want the hand edit kept", got)
	}

	// The record must still call the kept file a conflict next time, rather
	// than claim it was rewritten.
	changes, err := PlanApply(ApplyOptions{Templates: []Template{tpl}, Dest: dest})
	if err != nil {
		t.Fatalf("PlanApply() error = %v", err)
	}
	if got := statuses(changes); got["keep.txt"] != ChangeConflict || got["main.txt"] != ChangeUnchanged {
		t.Errorf("statuses = %v, want keep.txt still a conflict and main.txt settled", got)
	}
}

func TestApply_ForceRewritesWhatWasEdited(t *testing.T) {
	applyStore(t)
	dest := t.TempDir()
	tpl := create(t, dest, Values{"ci": true})
	if err := os.WriteFile(filepath.Join(dest, "keep.txt"), []byte("mine\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if _, err := Apply(ApplyOptions{Templates: []Template{tpl}, Dest: dest, Force: true}); err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if got := readFile(t, filepath.Join(dest, "keep.txt")); got != "steady\n" {
		t.Errorf("keep.txt = %q, want the template's content back", got)
	}
}

func TestApply_PruneDeletesWhatIsNoLongerWritten(t *testing.T) {
	applyStore(t)
	dest := t.TempDir()
	tpl := create(t, dest, Values{"ci": true})

	// Without --prune the file stays, and says so.
	changes, err := Apply(ApplyOptions{Templates: []Template{tpl}, Dest: dest, Values: Values{"ci": false}})
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if statuses(changes)["ci/job.yml"] != ChangeRemoved {
		t.Fatalf("statuses = %v, want ci/job.yml reported as removed", statuses(changes))
	}
	if _, err := os.Stat(filepath.Join(dest, "ci", "job.yml")); err != nil {
		t.Errorf("Stat(ci/job.yml) error = %v, want it kept without --prune", err)
	}

	if _, err := Apply(ApplyOptions{Templates: []Template{tpl}, Dest: dest, Values: Values{"ci": false}, Prune: true}); err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(dest, "ci", "job.yml")); !os.IsNotExist(err) {
		t.Errorf("Stat(ci/job.yml) error = %v, want it gone", err)
	}
	if _, err := os.Stat(filepath.Join(dest, "ci")); !os.IsNotExist(err) {
		t.Errorf("the emptied folder was left behind")
	}
	if _, err := os.Stat(filepath.Join(dest, "main.txt")); err != nil {
		t.Errorf("--prune removed more than it was asked to: %v", err)
	}
}

func TestApply_PruneNeverEmptiesTheProject(t *testing.T) {
	store := applyStore(t)
	dest := t.TempDir()
	tpl := create(t, dest, Values{"ci": false})

	// Every file leaves the template at once.
	writeTemplate(t, filepath.Join(store, "app", IgnoreFile), "*.txt\n")

	if _, err := Apply(ApplyOptions{Templates: []Template{tpl}, Dest: dest, Prune: true}); err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if _, err := os.Stat(dest); err != nil {
		t.Fatalf("the project folder itself was removed: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dest, RecordDir, RecordFile)); err != nil {
		t.Errorf("the record was removed with the files: %v", err)
	}
}

func TestPlanApply_ReplaysTheRecordedValues(t *testing.T) {
	store := applyStore(t)
	dest := t.TempDir()
	tpl := create(t, dest, Values{"ci": true, "greeting": "hi"})
	writeTemplate(t, filepath.Join(store, "app", "main.txt.tmpl"), "{{ .Values.greeting }} {{ .Name }}\n")

	// Nothing is passed this time: the values, and the name, come back from
	// the project's own record.
	if _, err := Apply(ApplyOptions{Templates: []Template{tpl}, Dest: dest}); err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if got := readFile(t, filepath.Join(dest, "main.txt")); got != "hi myapp\n" {
		t.Errorf("main.txt = %q, want the recorded values replayed", got)
	}
}

func TestPlanApply_RefusesAFileTemplate(t *testing.T) {
	store := useStore(t)
	writeTemplate(t, filepath.Join(store, "one.tmpl"), "x\n")
	tpl, err := Get("one")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	_, err = PlanApply(ApplyOptions{Templates: []Template{tpl}, Dest: t.TempDir()})
	if err == nil || !strings.Contains(err.Error(), "folder template") {
		t.Errorf("PlanApply() error = %v, want it to refuse a file template", err)
	}
}

func TestPlanApply_AProjectWithoutARecordIsAllConflicts(t *testing.T) {
	applyStore(t)
	dest := t.TempDir()
	tpl, err := Get("app")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	// A project created before there was a record, with a file of its own.
	if err := os.WriteFile(filepath.Join(dest, "main.txt"), []byte("theirs\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	changes, err := PlanApply(ApplyOptions{Templates: []Template{tpl}, Dest: dest, Name: "myapp"})
	if err != nil {
		t.Fatalf("PlanApply() error = %v", err)
	}
	got := statuses(changes)
	if got["main.txt"] != ChangeConflict {
		t.Errorf("main.txt = %s, want a file nobody recorded left alone", got["main.txt"])
	}
	if got["keep.txt"] != ChangeAdded {
		t.Errorf("keep.txt = %s, want the missing file added", got["keep.txt"])
	}
}

func TestTargets_DefaultsToWhatTheProjectRecords(t *testing.T) {
	store := applyStore(t)
	// A second template rendered into the same project, the way `t new
	// github-ci .` drops a workflow into something that already exists.
	writeTemplate(t, filepath.Join(store, "ci", "pipeline.yml.tmpl"), "on: push\n")

	dest := t.TempDir()
	create(t, dest, Values{"ci": true})
	second, err := Get("ci")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if _, err := Render(RenderOptions{Template: second, Dest: dest, Name: "myapp"}); err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	templates, err := Targets(ApplyOptions{Dest: dest})
	if err != nil {
		t.Fatalf("Targets() error = %v", err)
	}
	if len(templates) != 2 {
		t.Fatalf("Targets() = %v, want both templates the project records", templates)
	}

	changes, err := PlanApply(ApplyOptions{Dest: dest})
	if err != nil {
		t.Fatalf("PlanApply() error = %v", err)
	}
	seen := map[string]bool{}
	for _, change := range changes {
		seen[change.Template] = true
	}
	if !seen["app"] || !seen["ci"] {
		t.Errorf("PlanApply() covered %v, want both templates", seen)
	}
}

func TestTargets_SaysSoWhenTheProjectRecordsNothing(t *testing.T) {
	applyStore(t)

	_, err := Targets(ApplyOptions{Dest: t.TempDir()})
	if err == nil || !strings.Contains(err.Error(), "t new") {
		t.Errorf("Targets() error = %v, want it to say the project records nothing", err)
	}
}

func TestApply_AppliesEveryRecordedTemplate(t *testing.T) {
	store := applyStore(t)
	writeTemplate(t, filepath.Join(store, "ci", "pipeline.yml.tmpl"), "on: push\n")

	dest := t.TempDir()
	create(t, dest, Values{"ci": true})
	second, err := Get("ci")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if _, err := Render(RenderOptions{Template: second, Dest: dest, Name: "myapp"}); err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	// Both templates move on.
	writeTemplate(t, filepath.Join(store, "app", "main.txt.tmpl"), "hello {{ .Name }}, again\n")
	writeTemplate(t, filepath.Join(store, "ci", "pipeline.yml.tmpl"), "on: [push, pull_request]\n")

	if _, err := Apply(ApplyOptions{Dest: dest}); err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if got := readFile(t, filepath.Join(dest, "main.txt")); got != "hello myapp, again\n" {
		t.Errorf("main.txt = %q", got)
	}
	if got := readFile(t, filepath.Join(dest, "pipeline.yml")); got != "on: [push, pull_request]\n" {
		t.Errorf("pipeline.yml = %q", got)
	}
}
