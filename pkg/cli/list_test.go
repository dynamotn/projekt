package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func testView() ListView {
	view := ListView{Columns: []ListColumn{
		{Header: "NAME", Key: "name"},
		{Header: "TAGS", Key: "tags"},
	}}
	view.AppendRow("projekt", []string{"go", "work"})
	view.AppendRow("empty", []string(nil))
	return view
}

func TestParseOutputFormat(t *testing.T) {
	for _, name := range OutputFormats {
		format, err := ParseOutputFormat(name)
		if err != nil {
			t.Errorf("ParseOutputFormat(%q) error = %v", name, err)
		}
		if string(format) != name {
			t.Errorf("ParseOutputFormat(%q) = %v", name, format)
		}
	}

	if _, err := ParseOutputFormat("yaml"); err == nil {
		t.Error("ParseOutputFormat(\"yaml\") error = nil, want an error")
	}
}

func TestEncodeList_Table(t *testing.T) {
	var buf bytes.Buffer
	if err := EncodeList(&buf, testView(), ListOutputOption{NoColor: true}); err != nil {
		t.Fatalf("EncodeList() error = %v", err)
	}

	output := buf.String()
	for _, want := range []string{"NAME", "TAGS", "projekt", "go,work"} {
		if !strings.Contains(output, want) {
			t.Errorf("EncodeList() = %q, want it to contain %q", output, want)
		}
	}

	buf.Reset()
	if err := EncodeList(&buf, testView(), ListOutputOption{NoColor: true, NoHeaders: true}); err != nil {
		t.Fatalf("EncodeList() error = %v", err)
	}
	if strings.Contains(buf.String(), "NAME") {
		t.Errorf("EncodeList() with NoHeaders = %q, want no header row", buf.String())
	}
}

func TestEncodeList_TSV(t *testing.T) {
	var buf bytes.Buffer
	if err := EncodeList(&buf, testView(), ListOutputOption{Output: OutputTSV, NoHeaders: true}); err != nil {
		t.Fatalf("EncodeList() error = %v", err)
	}

	// One row per line, so `read` and `cut` work without stripping borders.
	want := "projekt\tgo,work\nempty\t\n"
	if buf.String() != want {
		t.Errorf("EncodeList() = %q, want %q", buf.String(), want)
	}
}

func TestEncodeList_JSON(t *testing.T) {
	var buf bytes.Buffer
	if err := EncodeList(&buf, testView(), ListOutputOption{Output: OutputJSON}); err != nil {
		t.Fatalf("EncodeList() error = %v", err)
	}

	var listing []map[string]any
	if err := json.Unmarshal(buf.Bytes(), &listing); err != nil {
		t.Fatalf("json.Unmarshal(%q) error = %v", buf.String(), err)
	}
	if len(listing) != 2 || listing[0]["name"] != "projekt" {
		t.Fatalf("EncodeList() = %#v", listing)
	}
	// An empty list stays [], never null.
	tags, ok := listing[1]["tags"].([]any)
	if !ok || len(tags) != 0 {
		t.Errorf("empty tags = %#v, want an empty array", listing[1]["tags"])
	}
}

func TestEncodeList_EmptyJSONIsAnArray(t *testing.T) {
	var buf bytes.Buffer
	view := ListView{Columns: []ListColumn{{Header: "NAME", Key: "name"}}}
	if err := EncodeList(&buf, view, ListOutputOption{Output: OutputJSON}); err != nil {
		t.Fatalf("EncodeList() error = %v", err)
	}
	if strings.TrimSpace(buf.String()) != "[]" {
		t.Errorf("EncodeList() = %q, want []", buf.String())
	}
}

func TestEncodeList_UnknownFormat(t *testing.T) {
	var buf bytes.Buffer
	if err := EncodeList(&buf, testView(), ListOutputOption{Output: OutputFormat("yaml")}); err == nil {
		t.Error("EncodeList() with an unknown format error = nil, want an error")
	}
}
