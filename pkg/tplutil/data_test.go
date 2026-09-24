package tplutil

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadData_StoreAndTemplate(t *testing.T) {
	store := useStore(t)
	writeTemplate(t, filepath.Join(store, DataFile), "company: Acme\nlicense: MIT\n")
	writeTemplate(t, filepath.Join(store, "go-cli", DataFile), "license: Apache-2.0\n")
	writeTemplate(t, filepath.Join(store, "go-cli", "README.md.tmpl"), "x\n")

	tpl, err := Get("go-cli")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	values, err := LoadData(tpl)
	if err != nil {
		t.Fatalf("LoadData() error = %v", err)
	}
	if values["company"] != "Acme" {
		t.Errorf("company = %v, want the store's Acme", values["company"])
	}
	if values["license"] != "Apache-2.0" {
		t.Errorf("license = %v, want the template's Apache-2.0", values["license"])
	}
}

func TestLoadData_JSONSpelling(t *testing.T) {
	store := useStore(t)
	writeTemplate(t, filepath.Join(store, ".data.json"), `{"company": "Acme"}`)
	writeTemplate(t, filepath.Join(store, "one.tmpl"), "x\n")

	tpl, err := Get("one")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	values, err := LoadData(tpl)
	if err != nil {
		t.Fatalf("LoadData() error = %v", err)
	}
	if values["company"] != "Acme" {
		t.Errorf("company = %v, want Acme", values["company"])
	}
}

func TestRender_DataIsADefaultTheUserCanOverride(t *testing.T) {
	store := useStore(t)
	writeTemplate(t, filepath.Join(store, DataFile), "author: Acme\nlicense: MIT\n")
	writeTemplate(t, filepath.Join(store, "license.tmpl"), "{{ .Values.author }} {{ .Values.license }}\n")

	tpl, err := Get("license")
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	dest := filepath.Join(t.TempDir(), "LICENSE")
	if _, err := Render(RenderOptions{Template: tpl, Dest: dest, Values: Values{"license": "BSD-3-Clause"}}); err != nil {
		t.Fatalf("Render() error = %v", err)
	}

	if got := strings.TrimSpace(readFile(t, dest)); got != "Acme BSD-3-Clause" {
		t.Errorf("rendered = %q, want the data file's author and the given licence", got)
	}
}

func TestList_SkipsDataFiles(t *testing.T) {
	store := useStore(t)
	writeTemplate(t, filepath.Join(store, "license.tmpl"), "x\n")
	writeTemplate(t, filepath.Join(store, "license.data.yaml"), "author: me\n")

	templates, err := List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(templates) != 1 || templates[0].Name != "license" {
		t.Errorf("List() = %v, want the licence template alone", templates)
	}
}
