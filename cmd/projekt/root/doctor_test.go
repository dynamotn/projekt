package root

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"gitlab.com/dynamo-tools/projekt/pkg/doctor"
)

func TestReportChecks_Text(t *testing.T) {
	checks := []doctor.Check{
		{Name: "git", Status: doctor.StatusOK, Detail: "git version 2.55.0"},
		{Name: "configuration", Status: doctor.StatusWarn, Detail: "nothing to jump to yet"},
	}

	var out bytes.Buffer
	if err := reportChecks(&out, checks, outputText, true); err != nil {
		t.Fatalf("reportChecks() error = %v", err)
	}
	for _, want := range []string{"[ok] git: git version", "[warn] configuration:"} {
		if !strings.Contains(out.String(), want) {
			t.Errorf("out = %q, want it to contain %q", out.String(), want)
		}
	}
}

func TestReportChecks_JSON(t *testing.T) {
	checks := []doctor.Check{
		{Name: "git", Status: doctor.StatusOK, Detail: "git version 2.55.0"},
		{Name: "binaries", Status: doctor.StatusFail, Detail: "not on PATH"},
	}

	var out bytes.Buffer
	if err := reportChecks(&out, checks, "json", true); err != nil {
		t.Fatalf("reportChecks() error = %v", err)
	}

	var report []map[string]any
	if err := json.Unmarshal(out.Bytes(), &report); err != nil {
		t.Fatalf("json.Unmarshal(%q) error = %v", out.String(), err)
	}
	if len(report) != 2 {
		t.Fatalf("report = %#v, want both checks", report)
	}
	// The order is the one they are worth reading in, not alphabetical.
	if report[0]["check"] != "git" || report[1]["check"] != "binaries" {
		t.Errorf("report = %#v, want the checks in order", report)
	}
	if report[1]["status"] != "fail" {
		t.Errorf("report = %#v", report[1])
	}
}

func TestReportChecks_UnknownFormat(t *testing.T) {
	var out bytes.Buffer
	err := reportChecks(&out, nil, "yaml", true)
	if err == nil {
		t.Fatal("reportChecks() error = nil, want an error")
	}
	// The message has to mention the format it forgot to list.
	if !strings.Contains(err.Error(), outputText) {
		t.Errorf("error = %v, want it to name %q as well", err, outputText)
	}
}

func TestDoctorCmd_Flags(t *testing.T) {
	var buf bytes.Buffer
	cmd := NewDoctorCmd(&buf)

	for _, name := range []string{"strict", "output", "no-color"} {
		if cmd.Flags().Lookup(name) == nil {
			t.Errorf("doctor is missing --%s", name)
		}
	}
	if got := cmd.Flags().Lookup("output").DefValue; got != outputText {
		t.Errorf("--output default = %q, want %q", got, outputText)
	}
}
