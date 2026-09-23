package cli

import "strings"

// FallbackEditor is used when neither VISUAL nor EDITOR is set. It is the one
// editor POSIX requires to be present.
const FallbackEditor = "vi"

// EditorCommand resolves the editor to run, honouring the usual environment
// variables. The value may carry arguments, as in `code --wait`.
//
// It lives here rather than beside one of its callers because opening the
// configuration and opening a project are the same question about the
// environment, and two answers to it would eventually differ.
func EditorCommand(env func(string) string) []string {
	for _, name := range []string{"VISUAL", "EDITOR"} {
		if value := strings.TrimSpace(env(name)); value != "" {
			return strings.Fields(value)
		}
	}
	return []string{FallbackEditor}
}
