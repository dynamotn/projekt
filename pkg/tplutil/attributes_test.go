package tplutil

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestParseAttributes(t *testing.T) {
	cases := []struct {
		name  string
		want  string
		attrs Attributes
	}{
		{"main.go", "main.go", Attributes{}},
		{"executable_build.sh", "build.sh", Attributes{Executable: true}},
		{"private_executable_deploy.sh", "deploy.sh", Attributes{Private: true, Executable: true}},
		{"readonly_LICENSE", "LICENSE", Attributes{Readonly: true}},
		{"symlink_latest", "latest", Attributes{Symlink: true}},
		{"dot_gitignore", ".gitignore", Attributes{}},
		{"literal_executable_x", "executable_x", Attributes{}},
	}
	for _, c := range cases {
		got, attrs := ParseAttributes(c.name)
		if got != c.want || attrs != c.attrs {
			t.Errorf("ParseAttributes(%q) = %q, %+v, want %q, %+v", c.name, got, attrs, c.want, c.attrs)
		}
	}
}

func TestAttributes_Modes(t *testing.T) {
	if got := (Attributes{}).FileMode(); got != 0o644 {
		t.Errorf("FileMode() = %o, want 644", got)
	}
	if got := (Attributes{Executable: true}).FileMode(); got != 0o755 {
		t.Errorf("FileMode(executable) = %o, want 755", got)
	}
	if got := (Attributes{Private: true, Executable: true}).FileMode(); got != 0o700 {
		t.Errorf("FileMode(private executable) = %o, want 700", got)
	}
	if got := (Attributes{Readonly: true}).FileMode(); got != 0o444 {
		t.Errorf("FileMode(readonly) = %o, want 444", got)
	}
	if got := (Attributes{Private: true}).DirMode(); got != 0o700 {
		t.Errorf("DirMode(private) = %o, want 700", got)
	}
}

func TestRender_AttributesShapeTheWrittenFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("file modes and symbolic links are a POSIX matter")
	}
	store := useStore(t)
	writeTemplate(t, filepath.Join(store, "app", "executable_{{ .Name }}.sh.tmpl"), "#!/bin/sh\necho {{ .Name }}\n")
	writeTemplate(t, filepath.Join(store, "app", "symlink_current.tmpl"), "{{ .Name }}.sh\n")
	writeTemplate(t, filepath.Join(store, "app", "private_secrets.env.tmpl"), "TOKEN=\n")

	tpl, err := Get("app")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	dest := t.TempDir()
	if _, err := Render(RenderOptions{Template: tpl, Dest: dest, Name: "myapp"}); err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	script := filepath.Join(dest, "myapp.sh")
	info, err := os.Stat(script)
	if err != nil {
		t.Fatalf("Stat(%s) error = %v", script, err)
	}
	if info.Mode().Perm() != 0o755 {
		t.Errorf("mode of %s = %o, want 755", script, info.Mode().Perm())
	}

	link, err := os.Readlink(filepath.Join(dest, "current"))
	if err != nil {
		t.Fatalf("Readlink() error = %v", err)
	}
	if link != "myapp.sh" {
		t.Errorf("link = %q, want myapp.sh", link)
	}

	secrets, err := os.Stat(filepath.Join(dest, "secrets.env"))
	if err != nil {
		t.Fatalf("Stat(secrets.env) error = %v", err)
	}
	if secrets.Mode().Perm() != 0o600 {
		t.Errorf("mode of secrets.env = %o, want 600", secrets.Mode().Perm())
	}
}

func TestRender_AValueCannotAddAnAttribute(t *testing.T) {
	store := useStore(t)
	writeTemplate(t, filepath.Join(store, "app", "{{ .Values.file }}.tmpl"), "x\n")

	tpl, err := Get("app")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	dest := t.TempDir()
	if _, err := Render(RenderOptions{
		Template: tpl,
		Dest:     dest,
		Name:     "app",
		Values:   Values{"file": "executable_run.sh"},
	}); err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	// The prefix is read off the template's own name, never off a value, so
	// this is a file honestly called executable_run.sh.
	info, err := os.Stat(filepath.Join(dest, "executable_run.sh"))
	if err != nil {
		t.Fatalf("Stat() error = %v", err)
	}
	if info.Mode().Perm()&0o111 != 0 {
		t.Errorf("mode = %o, want a value not to make a file executable", info.Mode().Perm())
	}
}
