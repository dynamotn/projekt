package config

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/spf13/viper"

	"gitlab.com/dynamo.foss/projekt/pkg/lazypath"
)

func TestEditorCommand(t *testing.T) {
	tests := []struct {
		name string
		env  map[string]string
		want []string
	}{
		{
			name: "falls back to vi when nothing is set",
			env:  map[string]string{},
			want: []string{"vi"},
		},
		{
			name: "EDITOR is used",
			env:  map[string]string{"EDITOR": "nano"},
			want: []string{"nano"},
		},
		{
			name: "VISUAL wins over EDITOR",
			env:  map[string]string{"VISUAL": "gvim", "EDITOR": "nano"},
			want: []string{"gvim"},
		},
		{
			name: "arguments are kept, so 'code --wait' works",
			env:  map[string]string{"EDITOR": "code --wait"},
			want: []string{"code", "--wait"},
		},
		{
			name: "a blank value is treated as unset",
			env:  map[string]string{"VISUAL": "   ", "EDITOR": "nano"},
			want: []string{"nano"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := editorCommand(func(key string) string { return tt.env[key] })
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("editorCommand() = %q, want %q", got, tt.want)
			}
		})
	}
}

// fakeEditor writes a script that replaces the edited file with content, and
// returns a command line for it.
func fakeEditor(t *testing.T, content string) string {
	t.Helper()

	dir := t.TempDir()
	payload := filepath.Join(dir, "payload.yaml")
	if err := os.WriteFile(payload, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	script := filepath.Join(dir, "editor.sh")
	body := "#!/bin/sh\ncat " + payload + " > \"$1\"\n"
	if err := os.WriteFile(script, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}

	return script
}

func TestRunConfigEdit(t *testing.T) {
	tests := []struct {
		name string
		// written is what the editor leaves in the file.
		written string
		wantErr string
		// wantReport is a substring the diagnostics must mention.
		wantReport string
	}{
		{
			name:    "a valid edit is accepted",
			written: "folders:\n  - path: /tmp\n",
		},
		{
			name:       "an edit that breaks the YAML is reported",
			written:    "folders:\n  - path: [unclosed\n",
			wantErr:    "no longer readable",
			wantReport: "",
		},
		{
			name:       "an edit that is valid YAML but invalid config is reported",
			written:    "folders:\n  - path: /tmp/ws\n    is_workspace: true\n    regex: '([a-z'\n",
			wantErr:    "1 error(s) after editing",
			wantReport: "invalid regex",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfgPath := filepath.Join(t.TempDir(), "config.yaml")
			if err := os.WriteFile(cfgPath, []byte("folders: []\n"), 0o644); err != nil {
				t.Fatal(err)
			}

			// viper and the cached config are process-wide, so put them back
			// the way the next test expects to find them.
			lazypath.CfgFile = cfgPath
			viper.Reset()
			t.Cleanup(func() {
				lazypath.CfgFile = ""
				viper.Reset()
				lazypath.ResetTestConfig()
			})

			lazypath.InitConfig()
			lazypath.ResetTestConfig()

			t.Setenv("VISUAL", "")
			t.Setenv("EDITOR", fakeEditor(t, tt.written))

			var out, errOut bytes.Buffer
			err := runConfigEdit(&out, strings.NewReader(""), &errOut)

			switch {
			case tt.wantErr == "" && err != nil:
				t.Fatalf("runConfigEdit() error = %v, want nil", err)
			case tt.wantErr != "" && err == nil:
				t.Fatalf("runConfigEdit() = nil, want an error mentioning %q", tt.wantErr)
			case tt.wantErr != "" && !strings.Contains(err.Error(), tt.wantErr):
				t.Fatalf("runConfigEdit() error = %v, want it to mention %q", err, tt.wantErr)
			}

			// The edit must actually have reached the file, or the test proves
			// nothing about reloading.
			onDisk, readErr := os.ReadFile(cfgPath)
			if readErr != nil {
				t.Fatal(readErr)
			}
			if string(onDisk) != tt.written {
				t.Errorf("config file = %q, want the edited content %q", onDisk, tt.written)
			}

			if tt.wantReport != "" && !strings.Contains(errOut.String(), tt.wantReport) {
				t.Errorf("diagnostics do not mention %q:\n%s", tt.wantReport, errOut.String())
			}
		})
	}
}

func TestRunConfigEdit_FailingEditorIsReported(t *testing.T) {
	cfgPath := filepath.Join(t.TempDir(), "config.yaml")
	if err := os.WriteFile(cfgPath, []byte("folders: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	lazypath.CfgFile = cfgPath
	viper.Reset()
	t.Cleanup(func() {
		lazypath.CfgFile = ""
		viper.Reset()
		lazypath.ResetTestConfig()
	})
	lazypath.InitConfig()

	t.Setenv("VISUAL", "")
	t.Setenv("EDITOR", "false")

	err := runConfigEdit(&bytes.Buffer{}, strings.NewReader(""), &bytes.Buffer{})
	if err == nil {
		t.Fatal("runConfigEdit() = nil, want the editor failure reported")
	}
	if !strings.Contains(err.Error(), "editor") {
		t.Errorf("error = %v, want it to name the editor", err)
	}
}

func TestNewConfigEditCmd_RunsOnAnUnreadableConfig(t *testing.T) {
	// Editing is how a broken config gets fixed, so it cannot be gated on the
	// config being readable.
	cmd := NewConfigEditCmd(&bytes.Buffer{})

	if cmd.PersistentPreRunE == nil {
		t.Fatal("config edit does not override the root PersistentPreRunE")
	}
	if err := cmd.PersistentPreRunE(cmd, nil); err != nil {
		t.Errorf("config edit PersistentPreRunE = %v, want nil", err)
	}
}
