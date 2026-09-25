package root

import (
	"os"
	"testing"
)

// TestMain keeps the trust decisions of the tests out of the real state
// directory, in both directions.
func TestMain(m *testing.M) {
	state, err := os.MkdirTemp("", "projekt-state-")
	if err != nil {
		panic(err)
	}
	os.Setenv("XDG_STATE_HOME", state)
	code := m.Run()
	os.RemoveAll(state)
	os.Exit(code)
}
