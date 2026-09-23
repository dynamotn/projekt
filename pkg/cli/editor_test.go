package cli

import (
	"reflect"
	"testing"
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
			got := EditorCommand(func(key string) string { return tt.env[key] })
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("EditorCommand() = %q, want %q", got, tt.want)
			}
		})
	}
}
