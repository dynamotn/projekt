package lazypath

import (
	"os"
	"path/filepath"
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
