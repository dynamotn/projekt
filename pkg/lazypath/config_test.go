package lazypath

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestConfigValidate(t *testing.T) {
	tmpDir := t.TempDir()
	existingPath := filepath.Join(tmpDir, "exists")
	os.Mkdir(existingPath, 0o755)

	tests := []struct {
		name    string
		config  Config
		wantErr bool
	}{
		{
			name: "valid config with existing paths",
			config: Config{
				Folders: []Folder{
					{Path: existingPath},
				},
			},
			wantErr: false,
		},
		{
			name: "config with empty path",
			config: Config{
				Folders: []Folder{
					{Path: ""},
				},
			},
			wantErr: true,
		},
		{
			name: "config with non-existent path",
			config: Config{
				Folders: []Folder{
					{Path: "/this/path/does/not/exist"},
				},
			},
			wantErr: false, // Just logs debug, doesn't error
		},
		{
			name: "empty config",
			config: Config{
				Folders: []Folder{},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("Config.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestGetConfig(t *testing.T) {
	// Test GetConfig with test config
	SetTestConfig(Config{
		Folders: []Folder{
			{Path: "/tmp/test", Prefix: "test"},
		},
	})
	defer ResetTestConfig()

	config := GetConfig()
	if len(config.Folders) != 1 {
		t.Errorf("GetConfig() returned %d folders, want 1", len(config.Folders))
	}

	if config.Folders[0].Path != "/tmp/test" {
		t.Errorf("GetConfig() folder path = %s, want /tmp/test", config.Folders[0].Path)
	}
}

func TestSetTestConfig(t *testing.T) {
	originalConfig := c

	testConfig := Config{
		Folders: []Folder{
			{Path: "/test/path", Prefix: "test"},
		},
		GitServers: []GitServer{
			{Name: "testserver", Type: "gitlab"},
		},
	}

	SetTestConfig(testConfig)

	if len(c.Folders) != 1 {
		t.Errorf("SetTestConfig() did not set folders correctly")
	}

	if len(c.GitServers) != 1 {
		t.Errorf("SetTestConfig() did not set git servers correctly")
	}

	// Reset
	c = originalConfig
}

func TestResetTestConfig(t *testing.T) {
	SetTestConfig(Config{
		Folders: []Folder{
			{Path: "/test"},
		},
	})

	ResetTestConfig()

	if len(c.Folders) != 0 {
		t.Errorf("ResetTestConfig() did not clear config, got %d folders", len(c.Folders))
	}
}

func TestInitConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")

	// Set config file for testing
	originalCfgFile := CfgFile
	CfgFile = configFile
	defer func() {
		CfgFile = originalCfgFile
	}()

	// Should create config file if not exists
	InitConfig()

	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		t.Error("InitConfig() did not create config file")
	}
}

func TestInitConfig_WithExistingFile(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")

	// Create config file first
	content := []byte("folders: []\n")
	if err := os.WriteFile(configFile, content, 0o644); err != nil {
		t.Fatal(err)
	}

	originalCfgFile := CfgFile
	CfgFile = configFile
	defer func() {
		CfgFile = originalCfgFile
	}()

	// Should read existing config file
	InitConfig()

	if _, err := os.Stat(configFile); err != nil {
		t.Errorf("InitConfig() failed with existing file: %v", err)
	}
}

func TestInitConfig_NoConfigFile(t *testing.T) {
	originalCfgFile := CfgFile
	CfgFile = ""
	defer func() {
		CfgFile = originalCfgFile
	}()

	// Should use default config path
	InitConfig()

	// Should have set CfgFile to default path
	if CfgFile == "" {
		t.Error("InitConfig() did not set default CfgFile")
	}
}

func TestUnmarshalConfig(t *testing.T) {
	// Setup test config
	SetTestConfig(Config{
		Folders: []Folder{
			{Path: "/tmp/test"},
		},
	})
	defer ResetTestConfig()

	// unmarshalConfig should not change already set config
	unmarshalConfig()

	if len(c.Folders) != 1 {
		t.Errorf("unmarshalConfig() changed config unexpectedly")
	}
}

func TestInitConfig_KeepsMalformedFile(t *testing.T) {
	tmpDir := t.TempDir()
	configFile := filepath.Join(tmpDir, "config.yaml")

	// A config file viper cannot parse must survive untouched.
	content := []byte("folders: [ this is not: valid: yaml\n")
	if err := os.WriteFile(configFile, content, 0o644); err != nil {
		t.Fatal(err)
	}

	originalCfgFile := CfgFile
	CfgFile = configFile
	defer func() {
		CfgFile = originalCfgFile
		loadErr = nil
	}()

	InitConfig()

	if LoadError() == nil {
		t.Error("LoadError() = nil, want the parse error that blocks any write")
	}

	folder := &Folder{Path: tmpDir}
	if err := folder.AddToConfig(); err == nil {
		t.Error("AddToConfig() = nil, want a refusal while the config is unreadable")
	}

	got, err := os.ReadFile(configFile)
	if err != nil {
		t.Fatalf("config file unreadable after InitConfig(): %v", err)
	}
	if string(got) != string(content) {
		t.Errorf("InitConfig() overwrote a malformed config file, got %q", string(got))
	}
}

func TestConfigValidate_Duplicates(t *testing.T) {
	config := Config{
		Folders: []Folder{
			{Path: "/tmp/dup"},
			{Path: "/tmp/dup/"},
		},
	}

	if err := config.Validate(); err == nil {
		t.Error("Config.Validate() = nil, want a duplicate folder error")
	}
}

func TestConfigValidate_InvalidRegex(t *testing.T) {
	config := Config{
		Folders: []Folder{
			{Path: "/tmp/ws", IsWorkspace: true, RegexMatch: "([a-z"},
		},
	}

	if err := config.Validate(); err == nil {
		t.Error("Config.Validate() = nil, want an invalid regex error")
	}
}

// diagnosticsOf returns the messages of one severity, so a test can assert on
// what was reported without depending on the order of the other severity.
func diagnosticsOf(diags []Diagnostic, severity Severity) []string {
	var messages []string
	for _, diag := range diags {
		if diag.Severity == severity {
			messages = append(messages, diag.Message)
		}
	}
	return messages
}

func TestConfigDiagnose(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name         string
		config       Config
		wantErrors   int
		wantWarnings int
		// containsError is a substring the error messages must mention.
		containsError string
	}{
		{
			name: "a clean config reports nothing",
			config: Config{
				GitServers: []GitServer{{Name: "github", HTTPS: "https://github.com"}},
				Folders:    []Folder{{Path: tmpDir}},
			},
		},
		{
			name: "a server with no URL at all is an error",
			config: Config{
				GitServers: []GitServer{{Name: "github"}},
			},
			wantErrors:    1,
			containsError: "neither an https nor an ssh URL",
		},
		{
			name: "a missing folder is only a warning",
			config: Config{
				Folders: []Folder{{Path: filepath.Join(tmpDir, "nope")}},
			},
			wantWarnings: 1,
		},
		{
			name: "a relative path warns without failing",
			config: Config{
				Folders: []Folder{{Path: "relative/path"}},
			},
			// The path is both relative and missing.
			wantWarnings: 2,
		},
		{
			name: "an unknown git server is a warning",
			config: Config{
				Folders: []Folder{{
					Path: tmpDir,
					Git:  &GitConfig{Host: "nowhere", Repos: []GitRepo{{Name: "a"}}},
				}},
			},
			wantWarnings: 1,
		},
		{
			name: "a repo with no name is an error",
			config: Config{
				GitServers: []GitServer{{Name: "github", HTTPS: "https://github.com"}},
				Folders: []Folder{{
					Path: tmpDir,
					Git:  &GitConfig{Host: "github", Repos: []GitRepo{{Name: ""}}},
				}},
			},
			wantErrors:    1,
			containsError: "empty name",
		},
		{
			name: "two repos checking out to the same place collide",
			config: Config{
				GitServers: []GitServer{{Name: "github", HTTPS: "https://github.com"}},
				Folders: []Folder{{
					Path: tmpDir,
					Git: &GitConfig{Host: "github", Repos: []GitRepo{
						{Name: "one", Path: "shared"},
						{Name: "two", Path: "shared"},
					}},
				}},
			},
			wantErrors:    1,
			containsError: "shared",
		},
		{
			name: "a remote with an empty URL is an error",
			config: Config{
				GitServers: []GitServer{{Name: "github", HTTPS: "https://github.com"}},
				Folders: []Folder{{
					Path: tmpDir,
					Git: &GitConfig{Host: "github", Repos: []GitRepo{
						{Name: "one", Remotes: map[string]string{"upstream": "  "}},
					}},
				}},
			},
			wantErrors:    1,
			containsError: "empty URL",
		},
		{
			name: "a worktree with no branch is an error",
			config: Config{
				GitServers: []GitServer{{Name: "github", HTTPS: "https://github.com"}},
				Folders: []Folder{{
					Path: tmpDir,
					Git: &GitConfig{Host: "github", Repos: []GitRepo{
						{Name: "one", Worktrees: []GitWorktree{{Path: "one-next"}}},
					}},
				}},
			},
			wantErrors:    1,
			containsError: "no branch",
		},
		{
			name: "a worktree with no path is an error",
			config: Config{
				GitServers: []GitServer{{Name: "github", HTTPS: "https://github.com"}},
				Folders: []Folder{{
					Path: tmpDir,
					Git: &GitConfig{Host: "github", Repos: []GitRepo{
						{Name: "one", Worktrees: []GitWorktree{{Branch: "next"}}},
					}},
				}},
			},
			wantErrors:    1,
			containsError: "empty path",
		},
		{
			name: "a worktree colliding with a repo checkout is an error",
			config: Config{
				GitServers: []GitServer{{Name: "github", HTTPS: "https://github.com"}},
				Folders: []Folder{{
					Path: tmpDir,
					Git: &GitConfig{Host: "github", Repos: []GitRepo{
						{Name: "one"},
						{Name: "two", Worktrees: []GitWorktree{{Path: "one", Branch: "next"}}},
					}},
				}},
			},
			wantErrors:    1,
			containsError: "worktree",
		},
		{
			name: "a valid remote and worktree report nothing",
			config: Config{
				GitServers: []GitServer{{Name: "github", HTTPS: "https://github.com"}},
				Folders: []Folder{{
					Path: tmpDir,
					Git: &GitConfig{Host: "github", Repos: []GitRepo{
						{
							Name:      "one",
							Remotes:   map[string]string{"upstream": "git@github.com:up/one.git"},
							Worktrees: []GitWorktree{{Path: "one-next", Branch: "next"}},
						},
					}},
				}},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			diags := tt.config.Diagnose()

			gotErrors := diagnosticsOf(diags, SeverityError)
			gotWarnings := diagnosticsOf(diags, SeverityWarning)

			if len(gotErrors) != tt.wantErrors {
				t.Errorf("got %d error(s) %q, want %d", len(gotErrors), gotErrors, tt.wantErrors)
			}
			if len(gotWarnings) != tt.wantWarnings {
				t.Errorf("got %d warning(s) %q, want %d", len(gotWarnings), gotWarnings, tt.wantWarnings)
			}

			if tt.containsError != "" {
				found := false
				for _, message := range gotErrors {
					if strings.Contains(message, tt.containsError) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("no error mentions %q, got %q", tt.containsError, gotErrors)
				}
			}

			// Validate is the error half of the same report, so the two must
			// never disagree about whether the config is usable.
			if err := tt.config.Validate(); (err != nil) != (tt.wantErrors > 0) {
				t.Errorf("Validate() = %v, but Diagnose() found %d error(s)", err, tt.wantErrors)
			}
		})
	}
}

func TestSeverityString(t *testing.T) {
	if got := SeverityError.String(); got != "ERROR" {
		t.Errorf("SeverityError = %q, want ERROR", got)
	}
	if got := SeverityWarning.String(); got != "WARNING" {
		t.Errorf("SeverityWarning = %q, want WARNING", got)
	}
}

func TestGitRepoRemoteNames(t *testing.T) {
	// The order has to be fixed, or sync and check would report the same
	// repository differently from one run to the next.
	repo := GitRepo{Remotes: map[string]string{
		"upstream": "u",
		"fork":     "f",
		"origin":   "o",
	}}

	want := []string{"fork", "origin", "upstream"}
	for range 10 {
		if got := repo.RemoteNames(); !reflect.DeepEqual(got, want) {
			t.Fatalf("RemoteNames() = %q, want %q", got, want)
		}
	}

	if got := (&GitRepo{}).RemoteNames(); got != nil {
		t.Errorf("RemoteNames() with no remotes = %q, want nil", got)
	}
}
