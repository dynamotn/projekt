package cli

import (
	"bytes"
	"strings"
	"testing"
)

func TestConfirm_WithoutATerminalAnswersNo(t *testing.T) {
	var errOut bytes.Buffer
	ok, err := Confirm(strings.NewReader("y\n"), &errOut, "Keep it? ")
	if err != nil || ok {
		t.Errorf("Confirm() = %v, %v, want no", ok, err)
	}
	if got := errOut.String(); got != "not saved; run again with --yes to keep it\n" {
		t.Errorf("errOut = %q, want how to keep it and no question", got)
	}
}
