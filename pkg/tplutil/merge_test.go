package tplutil

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMerge3(t *testing.T) {
	base := "a\nb\nc\nd\ne\n"
	tests := []struct {
		name         string
		ours, theirs string
		want         string
		clean        bool
	}{
		{"nothing changed", base, base, base, true},
		{"only ours changed", "a\nB\nc\nd\ne\n", base, "a\nB\nc\nd\ne\n", true},
		{"only theirs changed", base, "a\nb\nc\nD\ne\n", "a\nb\nc\nD\ne\n", true},
		{"both changed apart", "a\nB\nc\nd\ne\n", "a\nb\nc\nD\ne\n", "a\nB\nc\nD\ne\n", true},
		{"both changed alike", "a\nB\nc\nd\ne\n", "a\nB\nc\nd\ne\n", "a\nB\nc\nd\ne\n", true},
		{"ours added at the end, theirs at the start", base + "f\n", "z\n" + base, "z\n" + base + "f\n", true},
		{"theirs deleted, ours untouched there", "a\nb\nc\nd\ne\nmine\n", "a\nc\nd\ne\n", "a\nc\nd\ne\nmine\n", true},
		{"both changed the same line differently", "a\nB\nc\nd\ne\n", "a\nX\nc\nd\ne\n", "", false},
		{"ours deleted what theirs edited", "a\nc\nd\ne\n", "a\nX\nc\nd\ne\n", "", false},
		{"no final newline is kept", "a\nb\nc\nd\ne", "a\nB\nc\nd\ne\n", "a\nB\nc\nd\ne", true},
		{"adjacent changes are a conflict", "a\nb\nc\nd\nE\n", "a\nb\nc\nD\ne\n", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, clean := merge3([]byte(base), []byte(tt.ours), []byte(tt.theirs))
			if clean != tt.clean {
				t.Fatalf("clean = %v, want %v (got %q)", clean, tt.clean, got)
			}
			if clean && string(got) != tt.want {
				t.Errorf("merge3() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestMerge3_RefusesBinary(t *testing.T) {
	if _, clean := merge3([]byte("a\x00b"), []byte("a\x00c"), []byte("a\x00d")); clean {
		t.Error("a binary file must not be merged")
	}
}

func TestApply_MergesATemplateChangeIntoAnEditedFile(t *testing.T) {
	store := applyStore(t)
	dest := t.TempDir()
	writeTemplate(t, filepath.Join(store, "app", "config.txt.tmpl"), "one\ntwo\nthree\nfour\n")
	tpl := create(t, dest, Values{"ci": true})

	if _, err := os.Stat(filepath.Join(dest, RecordDir, BaseDir, "config.txt")); err != nil {
		t.Fatalf("the first render must keep a base: %v", err)
	}

	// The project edited the top, the template changed the bottom.
	if err := os.WriteFile(filepath.Join(dest, "config.txt"), []byte("ONE\ntwo\nthree\nfour\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeTemplate(t, filepath.Join(store, "app", "config.txt.tmpl"), "one\ntwo\nthree\nFOUR\n")

	changes, err := PlanApply(ApplyOptions{Templates: []Template{tpl}, Dest: dest})
	if err != nil {
		t.Fatalf("PlanApply() error = %v", err)
	}
	if got := statuses(changes)["config.txt"]; got != ChangeMerged {
		t.Fatalf("config.txt = %s, want merged", got)
	}

	if _, err := Apply(ApplyOptions{Templates: []Template{tpl}, Dest: dest}); err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if got := readFile(t, filepath.Join(dest, "config.txt")); got != "ONE\ntwo\nthree\nFOUR\n" {
		t.Errorf("config.txt = %q, want both changes", got)
	}
	// The base is the template's render, not the merge, so the next change to
	// the template merges against the right thing.
	if got := readFile(t, filepath.Join(dest, RecordDir, BaseDir, "config.txt")); got != "one\ntwo\nthree\nFOUR\n" {
		t.Errorf("base = %q, want the template's render", got)
	}

	// And the merged file counts as untouched: the next apply has nothing to do,
	// and a further template change merges again.
	changes, err = PlanApply(ApplyOptions{Templates: []Template{tpl}, Dest: dest})
	if err != nil {
		t.Fatalf("PlanApply() error = %v", err)
	}
	if got := statuses(changes)["config.txt"]; got != ChangeUnchanged {
		t.Errorf("after merge: config.txt = %s, want unchanged", got)
	}
	writeTemplate(t, filepath.Join(store, "app", "config.txt.tmpl"), "one\ntwo\nTHREE\nFOUR\n")
	if _, err := Apply(ApplyOptions{Templates: []Template{tpl}, Dest: dest}); err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if got := readFile(t, filepath.Join(dest, "config.txt")); got != "ONE\ntwo\nTHREE\nFOUR\n" {
		t.Errorf("config.txt = %q, want the second merge", got)
	}
}

func TestApply_OverlappingEditsStayAConflict(t *testing.T) {
	store := applyStore(t)
	dest := t.TempDir()
	writeTemplate(t, filepath.Join(store, "app", "config.txt.tmpl"), "one\ntwo\n")
	tpl := create(t, dest, Values{"ci": true})

	if err := os.WriteFile(filepath.Join(dest, "config.txt"), []byte("ONE\ntwo\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeTemplate(t, filepath.Join(store, "app", "config.txt.tmpl"), "UNO\ntwo\n")

	changes, err := Apply(ApplyOptions{Templates: []Template{tpl}, Dest: dest})
	if err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if got := statuses(changes)["config.txt"]; got != ChangeConflict {
		t.Errorf("config.txt = %s, want conflict", got)
	}
	if got := readFile(t, filepath.Join(dest, "config.txt")); got != "ONE\ntwo\n" {
		t.Errorf("config.txt = %q, want the hand edit kept, no markers", got)
	}
}

func TestApply_EditThatMatchesTheTemplateIsUnchanged(t *testing.T) {
	store := applyStore(t)
	dest := t.TempDir()
	tpl := create(t, dest, Values{"ci": true})

	// The hand edit is exactly what the template now says.
	if err := os.WriteFile(filepath.Join(dest, "keep.txt"), []byte("steadier\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeTemplate(t, filepath.Join(store, "app", "keep.txt.tmpl"), "steadier\n")

	changes, err := PlanApply(ApplyOptions{Templates: []Template{tpl}, Dest: dest})
	if err != nil {
		t.Fatalf("PlanApply() error = %v", err)
	}
	if got := statuses(changes)["keep.txt"]; got != ChangeUnchanged {
		t.Errorf("keep.txt = %s, want unchanged", got)
	}
}

func TestApply_AProjectWithoutABaseGainsOne(t *testing.T) {
	applyStore(t)
	dest := t.TempDir()
	tpl := create(t, dest, Values{"ci": true})
	if err := os.RemoveAll(filepath.Join(dest, RecordDir, BaseDir)); err != nil {
		t.Fatal(err)
	}

	if _, err := Apply(ApplyOptions{Templates: []Template{tpl}, Dest: dest}); err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(dest, RecordDir, BaseDir, "main.txt")); err != nil {
		t.Errorf("an unchanged file must get its base back: %v", err)
	}
}

func TestApply_PruneForgetsTheBase(t *testing.T) {
	applyStore(t)
	dest := t.TempDir()
	tpl := create(t, dest, Values{"ci": true})

	if _, err := Apply(ApplyOptions{Templates: []Template{tpl}, Dest: dest, Values: Values{"ci": false}, Prune: true}); err != nil {
		t.Fatalf("Apply() error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(dest, RecordDir, BaseDir, "ci")); !os.IsNotExist(err) {
		t.Errorf("Stat(base/ci) error = %v, want it gone with the file", err)
	}
}
