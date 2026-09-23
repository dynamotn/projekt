package tplutil

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestParseSet(t *testing.T) {
	values, err := ParseSet([]string{
		"name=projekt",
		"port=8080",
		"debug=true",
		"author.name=Jane Doe",
		"author.mail=jane@example.com",
		"empty=",
		"url=https://example.com/a=b",
	})
	if err != nil {
		t.Fatalf("ParseSet() error = %v", err)
	}

	if values["name"] != "projekt" {
		t.Errorf("name = %#v, want projekt", values["name"])
	}
	if values["port"] != 8080 {
		t.Errorf("port = %#v, want int 8080", values["port"])
	}
	if values["debug"] != true {
		t.Errorf("debug = %#v, want bool true", values["debug"])
	}
	if values["empty"] != "" {
		t.Errorf("empty = %#v, want empty string", values["empty"])
	}
	// Only the first "=" separates the key: the rest belongs to the value.
	if values["url"] != "https://example.com/a=b" {
		t.Errorf("url = %#v, want the full URL", values["url"])
	}

	author, ok := values["author"].(Values)
	if !ok {
		t.Fatalf("author = %#v, want a nested map", values["author"])
	}
	if author["name"] != "Jane Doe" || author["mail"] != "jane@example.com" {
		t.Errorf("author = %#v, want both nested keys", author)
	}
}

func TestParseSet_Errors(t *testing.T) {
	for _, assignment := range []string{"novalue", "=value", "a.=1", "a=1"} {
		sets := []string{assignment}
		if assignment == "a=1" {
			// A scalar cannot also be a map.
			sets = []string{"a=1", "a.b=2"}
		}
		if _, err := ParseSet(sets); err == nil {
			t.Errorf("ParseSet(%v) error = nil, want an error", sets)
		}
	}
}

func TestLoadValuesFiles(t *testing.T) {
	dir := t.TempDir()
	first := filepath.Join(dir, "first.yaml")
	second := filepath.Join(dir, "second.yaml")
	if err := os.WriteFile(first, []byte("name: one\nauthor:\n  name: Jane\n  mail: jane@example.com\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if err := os.WriteFile(second, []byte("name: two\nauthor:\n  mail: two@example.com\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	values, err := LoadValuesFiles([]string{first, second})
	if err != nil {
		t.Fatalf("LoadValuesFiles() error = %v", err)
	}
	if values["name"] != "two" {
		t.Errorf("name = %#v, want two (the last file wins)", values["name"])
	}
	author, ok := values["author"].(Values)
	if !ok {
		t.Fatalf("author = %#v, want a nested map", values["author"])
	}
	// The second file only overrides one nested key, the other survives.
	if author["name"] != "Jane" || author["mail"] != "two@example.com" {
		t.Errorf("author = %#v, want a deep merge", author)
	}
}

func TestLoadValuesFiles_Errors(t *testing.T) {
	dir := t.TempDir()
	broken := filepath.Join(dir, "broken.yaml")
	if err := os.WriteFile(broken, []byte("name: [unterminated\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	if _, err := LoadValuesFiles([]string{filepath.Join(dir, "missing.yaml")}); err == nil {
		t.Error("LoadValuesFiles() with a missing file error = nil, want an error")
	}
	if _, err := LoadValuesFiles([]string{broken}); err == nil {
		t.Error("LoadValuesFiles() with a broken file error = nil, want an error")
	}
}

func TestMergeValues(t *testing.T) {
	base := map[string]any{"a": 1, "nested": map[string]any{"x": 1, "y": 2}}
	override := map[string]any{"b": 2, "nested": map[string]any{"y": 3}}

	merged := MergeValues(base, override)
	want := Values{"a": 1, "b": 2, "nested": Values{"x": 1, "y": 3}}
	if !reflect.DeepEqual(merged, want) {
		t.Errorf("MergeValues() = %#v, want %#v", merged, want)
	}
	// The inputs must not be touched.
	if len(base) != 2 || len(override) != 2 {
		t.Error("MergeValues() modified its arguments")
	}
}
