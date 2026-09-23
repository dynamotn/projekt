package folderutil

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"gitlab.com/dynamo.foss/projekt/pkg/lazypath"
)

func TestFindFolderByShortName(t *testing.T) {
	tmpDir := t.TempDir()
	testProject := filepath.Join(tmpDir, "test-project")
	if err := os.Mkdir(testProject, 0o755); err != nil {
		t.Fatal(err)
	}

	// Mock config
	lazypath.SetTestConfig(lazypath.Config{
		Folders: []lazypath.Folder{
			{
				Path:        tmpDir,
				Prefix:      "tmp",
				IsWorkspace: true,
				RegexMatch:  "test-.*",
			},
		},
	})
	defer lazypath.ResetTestConfig()

	tests := []struct {
		name      string
		shortName string
		wantPath  string
		wantErr   bool
	}{
		{
			name:      "existing folder",
			shortName: "tmp-test-project",
			wantPath:  testProject,
			wantErr:   false,
		},
		{
			name:      "non-existing folder",
			shortName: "nonexistent",
			wantPath:  "",
			wantErr:   true, // Unknown short names must be reported as an error
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			err := FindFolderByShortName(&buf, tt.shortName, GetOptions{NoRecord: true})

			if (err != nil) != tt.wantErr {
				t.Errorf("FindFolderByShortName() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			output := strings.TrimSpace(buf.String())
			if tt.wantPath != "" && output != tt.wantPath {
				t.Errorf("FindFolderByShortName() output = %v, want %v", output, tt.wantPath)
			}
		})
	}
}

func TestFindFolderByShortName_EmptyConfig(t *testing.T) {
	// Test with empty config
	lazypath.SetTestConfig(lazypath.Config{
		Folders: []lazypath.Folder{},
	})
	defer lazypath.ResetTestConfig()

	var buf bytes.Buffer
	err := FindFolderByShortName(&buf, "any-name", GetOptions{NoRecord: true})
	if !errors.Is(err, ErrFolderNotFound) {
		t.Errorf("FindFolderByShortName() error = %v, want ErrFolderNotFound", err)
	}

	output := strings.TrimSpace(buf.String())
	if output != "" {
		t.Errorf("FindFolderByShortName() should return empty path, got: %s", output)
	}
}

func TestImportFolderToConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")

	// Setup test config
	lazypath.CfgFile = configFile
	lazypath.InitConfig()

	tests := []struct {
		name    string
		folder  *lazypath.Folder
		wantErr bool
	}{
		{
			name: "add new folder",
			folder: &lazypath.Folder{
				Path:        "/tmp/test1",
				Prefix:      "test",
				IsWorkspace: false,
			},
			wantErr: false,
		},
		{
			name: "add folder with workspace",
			folder: &lazypath.Folder{
				Path:        "/tmp/workspace",
				Prefix:      "ws",
				IsWorkspace: true,
				RegexMatch:  "^[^.].+",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ImportFolderToConfig(tt.folder)
			if (err != nil) != tt.wantErr {
				t.Errorf("ImportFolderToConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestListFolders(t *testing.T) {
	tmpDir := t.TempDir()
	testProject := filepath.Join(tmpDir, "test-project")
	if err := os.Mkdir(testProject, 0o755); err != nil {
		t.Fatal(err)
	}

	// Mock config
	lazypath.SetTestConfig(lazypath.Config{
		Folders: []lazypath.Folder{
			{
				Path:        tmpDir,
				Prefix:      "tmp",
				IsWorkspace: true,
				RegexMatch:  "test-.*",
			},
			{
				Path:        "/home/user/project",
				Prefix:      "home",
				IsWorkspace: false,
			},
		},
	})
	defer lazypath.ResetTestConfig()

	tests := []struct {
		name    string
		option  *ListOption
		wantErr bool
		check   func(string) bool
	}{
		{
			name: "list all folders with default options",
			option: &ListOption{
				IsPlain:   false,
				ShortOnly: false,
				NoHeaders: false,
				NoColor:   true,
			},
			wantErr: false,
			check: func(output string) bool {
				return strings.Contains(output, "SHORT NAME")
			},
		},
		{
			name: "list plain folders",
			option: &ListOption{
				IsPlain:   true,
				ShortOnly: false,
				NoHeaders: false,
				NoColor:   true,
			},
			wantErr: false,
			check: func(output string) bool {
				return strings.Contains(output, "PATH")
			},
		},
		{
			name: "list plain folders without headers",
			option: &ListOption{
				IsPlain:   true,
				ShortOnly: false,
				NoHeaders: true,
				NoColor:   true,
			},
			wantErr: false,
			check: func(output string) bool {
				return len(output) > 0
			},
		},
		{
			name: "list short names only",
			option: &ListOption{
				IsPlain:   false,
				ShortOnly: true,
				NoHeaders: false,
				NoColor:   true,
			},
			wantErr: false,
			check: func(output string) bool {
				return strings.Contains(output, "SHORT NAME")
			},
		},
		{
			name: "list without headers",
			option: &ListOption{
				IsPlain:   false,
				ShortOnly: false,
				NoHeaders: true,
				NoColor:   true,
			},
			wantErr: false,
			check: func(output string) bool {
				return !strings.Contains(output, "SHORT NAME") || len(output) > 0
			},
		},
		{
			name: "list with color",
			option: &ListOption{
				IsPlain:   false,
				ShortOnly: false,
				NoHeaders: false,
				NoColor:   false,
			},
			wantErr: false,
			check: func(output string) bool {
				return len(output) > 0
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			err := ListFolders(&buf, tt.option)

			if (err != nil) != tt.wantErr {
				t.Errorf("ListFolders() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			output := buf.String()
			if !tt.wantErr && tt.check != nil && !tt.check(output) {
				t.Errorf("ListFolders() output check failed, got: %s", output)
			}
		})
	}
}

// setTwoFolderConfig installs a config of two plain folders, which needs no
// filesystem and keeps the listing order fixed (the priority sort is stable).
func setTwoFolderConfig(t *testing.T) {
	t.Helper()

	lazypath.SetTestConfig(lazypath.Config{
		Folders: []lazypath.Folder{
			{Path: "/home/user/alpha", Prefix: "work", IsWorkspace: false, Tags: []string{"go", "work"}},
			{Path: "/home/user/beta", IsWorkspace: false, Priority: 7},
		},
	})
	t.Cleanup(lazypath.ResetTestConfig)
}

func TestListFolders_JSON(t *testing.T) {
	setTwoFolderConfig(t)

	var buf bytes.Buffer
	if err := ListFolders(&buf, &ListOption{Output: OutputJSON}); err != nil {
		t.Fatalf("ListFolders() error = %v", err)
	}

	var got []map[string]any
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, buf.String())
	}

	// beta sorts first: it has the higher priority. An untagged folder carries
	// an empty array rather than null.
	want := []map[string]any{
		{"shortName": "beta", "path": "/home/user/beta", "workspace": "/home/user/beta", "tags": []any{}},
		{"shortName": "work-alpha", "path": "/home/user/alpha", "workspace": "/home/user/alpha", "tags": []any{"go", "work"}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ListFolders(json) =\n%#v\nwant\n%#v", got, want)
	}
}

func TestListFolders_JSONShortOnly(t *testing.T) {
	setTwoFolderConfig(t)

	var buf bytes.Buffer
	if err := ListFolders(&buf, &ListOption{Output: OutputJSON, ShortOnly: true}); err != nil {
		t.Fatalf("ListFolders() error = %v", err)
	}

	var got []map[string]any
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, buf.String())
	}

	want := []map[string]any{
		{"shortName": "beta"},
		{"shortName": "work-alpha"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("ListFolders(json, short-only) =\n%#v\nwant\n%#v", got, want)
	}
}

func TestListFolders_JSONPlainKeepsTypes(t *testing.T) {
	// A table stringifies everything; JSON must not, or a consumer has to parse
	// "true" and "7" back out again.
	setTwoFolderConfig(t)

	var buf bytes.Buffer
	if err := ListFolders(&buf, &ListOption{Output: OutputJSON, IsPlain: true}); err != nil {
		t.Fatalf("ListFolders() error = %v", err)
	}

	var got []map[string]any
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, buf.String())
	}
	if len(got) != 2 {
		t.Fatalf("got %d folders, want 2", len(got))
	}

	// The plain view lists folders as configured, so alpha stays first.
	if isWorkspace, ok := got[0]["isWorkspace"].(bool); !ok || isWorkspace {
		t.Errorf("isWorkspace = %#v, want the bool false", got[0]["isWorkspace"])
	}
	if priority, ok := got[1]["priority"].(float64); !ok || priority != 7 {
		t.Errorf("priority = %#v, want the number 7", got[1]["priority"])
	}
}

func TestListFolders_JSONEmptyIsArray(t *testing.T) {
	// A consumer should never have to handle null in place of an empty list.
	//
	// An empty workspace is what produces an empty listing here: an entirely
	// empty lazypath.Config cannot, because it is indistinguishable from an
	// unset one and sends the loader back to whatever viper still holds.
	lazypath.SetTestConfig(lazypath.Config{
		Folders: []lazypath.Folder{
			{Path: t.TempDir(), IsWorkspace: true},
		},
	})
	t.Cleanup(lazypath.ResetTestConfig)

	var buf bytes.Buffer
	if err := ListFolders(&buf, &ListOption{Output: OutputJSON}); err != nil {
		t.Fatalf("ListFolders() error = %v", err)
	}

	if got := strings.TrimSpace(buf.String()); got != "[]" {
		t.Errorf("ListFolders(json) with no folders = %q, want %q", got, "[]")
	}
}

func TestListFolders_TSV(t *testing.T) {
	setTwoFolderConfig(t)

	tests := []struct {
		name      string
		option    *ListOption
		wantLines []string
	}{
		{
			name:   "short names without headers",
			option: &ListOption{Output: OutputTSV, ShortOnly: true, NoHeaders: true},
			wantLines: []string{
				"beta",
				"work-alpha",
			},
		},
		{
			name:   "full listing with headers",
			option: &ListOption{Output: OutputTSV},
			wantLines: []string{
				"SHORT NAME\tPATH\tWORKSPACE PATH\tTAGS",
				"beta\t/home/user/beta\t/home/user/beta\t",
				"work-alpha\t/home/user/alpha\t/home/user/alpha\tgo,work",
			},
		},
		{
			name:   "filtered to one tag",
			option: &ListOption{Output: OutputTSV, ShortOnly: true, NoHeaders: true, Tags: []string{"go"}},
			wantLines: []string{
				"work-alpha",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			if err := ListFolders(&buf, tt.option); err != nil {
				t.Fatalf("ListFolders() error = %v", err)
			}

			got := strings.Split(strings.TrimSuffix(buf.String(), "\n"), "\n")
			if !reflect.DeepEqual(got, tt.wantLines) {
				t.Errorf("ListFolders(tsv) =\n%q\nwant\n%q", got, tt.wantLines)
			}
		})
	}
}

func TestListFolders_TagFilter(t *testing.T) {
	setTwoFolderConfig(t)

	tests := []struct {
		name   string
		tags   []string
		option func(*ListOption)
		want   []string
	}{
		{
			name: "no tags lists everything",
			want: []string{"beta", "work-alpha"},
		},
		{
			name: "one matching tag",
			tags: []string{"work"},
			want: []string{"work-alpha"},
		},
		{
			name: "every tag must match",
			tags: []string{"go", "work"},
			want: []string{"work-alpha"},
		},
		{
			name: "one missing tag excludes the folder",
			tags: []string{"go", "rust"},
			want: nil,
		},
		{
			name: "an unknown tag matches nothing",
			tags: []string{"nope"},
			want: nil,
		},
		{
			name: "blank tags are ignored, not treated as a filter",
			tags: []string{"  ", ""},
			want: []string{"beta", "work-alpha"},
		},
		{
			name: "surrounding whitespace is forgiven",
			tags: []string{" work "},
			want: []string{"work-alpha"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			option := &ListOption{Output: OutputTSV, ShortOnly: true, NoHeaders: true, Tags: tt.tags}
			if err := ListFolders(&buf, option); err != nil {
				t.Fatalf("ListFolders() error = %v", err)
			}

			var got []string
			if trimmed := strings.TrimSpace(buf.String()); trimmed != "" {
				got = strings.Split(trimmed, "\n")
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ListFolders(tags=%q) = %q, want %q", tt.tags, got, tt.want)
			}
		})
	}
}

func TestParseConfig_WorkspaceChildrenInheritTags(t *testing.T) {
	// A folder inside a workspace has no configuration entry of its own, so the
	// only tags it can have are the workspace's.
	tmpDir := t.TempDir()
	for _, name := range []string{"one", "two"} {
		if err := os.Mkdir(filepath.Join(tmpDir, name), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	folders, err := ParseConfig(lazypath.Config{
		Folders: []lazypath.Folder{
			{Path: tmpDir, IsWorkspace: true, Tags: []string{"oss", "go"}},
		},
	})
	if err != nil {
		t.Fatalf("ParseConfig() error = %v", err)
	}
	if len(folders) != 2 {
		t.Fatalf("got %d folders, want 2", len(folders))
	}

	for _, folder := range folders {
		if !reflect.DeepEqual(folder.Tags, []string{"oss", "go"}) {
			t.Errorf("%s tags = %q, want the workspace's tags", folder.ShortName, folder.Tags)
		}
	}

	if got := FilterByTags(folders, []string{"oss"}); len(got) != 2 {
		t.Errorf("FilterByTags(oss) kept %d folders, want 2", len(got))
	}
	if got := FilterByTags(folders, []string{"rust"}); len(got) != 0 {
		t.Errorf("FilterByTags(rust) kept %d folders, want 0", len(got))
	}
}

func TestListFolders_UnknownOutputFormat(t *testing.T) {
	setTwoFolderConfig(t)

	var buf bytes.Buffer
	err := ListFolders(&buf, &ListOption{Output: OutputFormat("yaml")})
	if err == nil {
		t.Fatal("ListFolders() with an unknown format returned no error")
	}
	if !strings.Contains(err.Error(), "yaml") {
		t.Errorf("error %q does not name the rejected format", err)
	}
}

func TestParseOutputFormat(t *testing.T) {
	for _, name := range OutputFormats {
		t.Run(name, func(t *testing.T) {
			got, err := ParseOutputFormat(name)
			if err != nil {
				t.Fatalf("ParseOutputFormat(%q) error = %v", name, err)
			}
			if string(got) != name {
				t.Errorf("ParseOutputFormat(%q) = %q", name, got)
			}
		})
	}

	t.Run("rejects an unknown format", func(t *testing.T) {
		if _, err := ParseOutputFormat("xml"); err == nil {
			t.Error("ParseOutputFormat(\"xml\") returned no error")
		}
	})

	t.Run("rejects the empty string", func(t *testing.T) {
		// The flag always has a value, so an empty one is a caller mistake
		// rather than a request for the default.
		if _, err := ParseOutputFormat(""); err == nil {
			t.Error("ParseOutputFormat(\"\") returned no error")
		}
	})
}

func TestRemoveFolderFromConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")

	// Setup test config
	lazypath.CfgFile = configFile
	lazypath.InitConfig()

	// Add a folder first
	folder := &lazypath.Folder{
		Path:        "/tmp/test-remove",
		Prefix:      "test",
		IsWorkspace: false,
	}
	ImportFolderToConfig(folder)

	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{
			name:    "remove existing folder",
			path:    "/tmp/test-remove",
			wantErr: false,
		},
		{
			name:    "remove non-existing folder",
			path:    "/tmp/nonexistent",
			wantErr: false, // Should not error, just log
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := RemoveFolderFromConfig(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("RemoveFolderFromConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestParseConfig_SymlinkHandling(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a regular directory
	regularDir := filepath.Join(tmpDir, "regular-dir")
	if err := os.Mkdir(regularDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Create a file (not a directory)
	regularFile := filepath.Join(tmpDir, "regular-file")
	if err := os.WriteFile(regularFile, []byte("content"), 0o644); err != nil {
		t.Fatal(err)
	}

	config := lazypath.Config{
		Folders: []lazypath.Folder{
			{
				Path:        tmpDir,
				IsWorkspace: true,
				RegexMatch:  ".*",
			},
		},
	}

	result, err := ParseConfig(config)
	if err != nil {
		t.Fatalf("ParseConfig() error = %v", err)
	}

	// Should include regular-dir but not regular-file
	found := false
	for _, folder := range result {
		if strings.Contains(folder.ShortName, "regular-dir") {
			found = true
		}
		if strings.Contains(folder.ShortName, "regular-file") {
			t.Error("ParseConfig() should not include regular files")
		}
	}

	if !found {
		t.Error("ParseConfig() should include regular directories")
	}
}

func TestParseConfig_ErrorHandling(t *testing.T) {
	tests := []struct {
		name    string
		config  lazypath.Config
		wantErr bool
	}{
		{
			name: "workspace with non-existent path",
			config: lazypath.Config{
				Folders: []lazypath.Folder{
					{
						Path:        "/nonexistent/path",
						IsWorkspace: true,
						RegexMatch:  ".*",
					},
				},
			},
			wantErr: false, // Should not error, just skip
		},
		{
			name: "workspace with empty prefix",
			config: lazypath.Config{
				Folders: []lazypath.Folder{
					{
						Path:        "/tmp",
						Prefix:      "",
						IsWorkspace: true,
					},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseConfig(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
