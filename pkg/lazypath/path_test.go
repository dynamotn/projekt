package lazypath

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNormalizePath(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("no home directory available")
	}
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name    string
		path    string
		want    string
		wantErr bool
	}{
		{name: "absolute path", path: "/tmp/projects", want: "/tmp/projects"},
		{name: "trailing slash", path: "/tmp/projects/", want: "/tmp/projects"},
		{name: "dot segments", path: "/tmp/projects/../projects", want: "/tmp/projects"},
		{name: "home shortcut", path: "~/code", want: filepath.Join(home, "code")},
		{name: "bare home", path: "~", want: home},
		{name: "relative path", path: "code", want: filepath.Join(cwd, "code")},
		{name: "empty path", path: "   ", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NormalizePath(tt.path)
			if (err != nil) != tt.wantErr {
				t.Fatalf("NormalizePath(%q) error = %v, wantErr %v", tt.path, err, tt.wantErr)
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("NormalizePath(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestCheckFolderExist_DifferentSpellings(t *testing.T) {
	SetTestConfig(Config{
		Folders: []Folder{
			{Path: "/tmp/projects/"},
		},
	})
	defer ResetTestConfig()

	for _, path := range []string{"/tmp/projects", "/tmp/projects/", "/tmp/projects/./"} {
		if exist, _ := CheckFolderExist(path); !exist {
			t.Errorf("CheckFolderExist(%q) = false, want true", path)
		}
	}

	if exist, _ := CheckFolderExist("/tmp/other"); exist {
		t.Error("CheckFolderExist(/tmp/other) = true, want false")
	}
}

func TestCanonical_ResolvesLinksAndKeepsTheMissingTail(t *testing.T) {
	root := t.TempDir()
	real := filepath.Join(root, "real")
	if err := os.MkdirAll(real, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "link")
	if err := os.Symlink(real, link); err != nil {
		t.Skipf("cannot create a symbolic link here: %v", err)
	}

	if !SamePath(link, real) {
		t.Errorf("SamePath(%q, %q) = false, want the link to name the folder", link, real)
	}
	want := filepath.Join(Canonical(real), "not", "yet")
	if got := Canonical(filepath.Join(link, "not", "yet")); got != want {
		t.Errorf("Canonical() = %q, want %q: the existing prefix resolved, the rest kept", got, want)
	}
	if SamePath(real, filepath.Join(real, "other")) {
		t.Error("SamePath() = true for two different folders")
	}
}
