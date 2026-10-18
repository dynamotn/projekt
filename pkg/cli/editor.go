package cli

import "strings"

// FallbackEditor is used when neither VISUAL nor EDITOR is set. It is the one
// editor POSIX requires to be present.
const FallbackEditor = "vi"

// EditorCommand resolves the editor to run, honouring the usual environment
// variables. The value may carry arguments, as in `code --wait`, and quotes,
// as in `"/Applications/Some Editor.app/bin/editor" -w`.
//
// It lives here rather than beside one of its callers because opening the
// configuration and opening a project are the same question about the
// environment, and two answers to it would eventually differ.
func EditorCommand(env func(string) string) []string {
	for _, name := range []string{"VISUAL", "EDITOR"} {
		if value := strings.TrimSpace(env(name)); value != "" {
			if words := SplitShellWords(value); len(words) > 0 {
				return words
			}
		}
	}
	return []string{FallbackEditor}
}

// SplitShellWords splits a command line the way a POSIX shell splits words:
// on blanks, except inside single or double quotes, with a backslash
// escaping the next character outside single quotes. It does no expansion.
func SplitShellWords(line string) []string {
	var (
		words   []string
		current strings.Builder
		inWord  bool
		quote   rune
		escaped bool
	)
	for _, r := range line {
		switch {
		case escaped:
			current.WriteRune(r)
			escaped = false
		case quote == '\'' && r != '\'':
			current.WriteRune(r)
		case r == '\\' && quote != '\'':
			escaped, inWord = true, true
		case quote != 0 && r == quote:
			quote = 0
		case quote == 0 && (r == '\'' || r == '"'):
			quote, inWord = r, true
		case quote == 0 && (r == ' ' || r == '\t' || r == '\n'):
			if inWord {
				words = append(words, current.String())
				current.Reset()
				inWord = false
			}
		default:
			current.WriteRune(r)
			inWord = true
		}
	}
	if inWord {
		words = append(words, current.String())
	}
	return words
}
