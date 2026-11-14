package folderutil

import (
	"fmt"
	"io"
	"os/exec"
	"strings"

	"gitlab.com/dynamo-tools/projekt/pkg/aiutil"
	"gitlab.com/dynamo-tools/projekt/pkg/cli"
)

// GenerateOptions say how GenerateDescription talks to the owner and to the
// AI command.
type GenerateOptions struct {
	ShowPrompt bool      // print the prompt and stop
	Yes        bool      // keep the answer without asking
	In         io.Reader // the terminal, for the question
	Out, Err   io.Writer
	// Ask is the AI call; nil means aiutil.Ask. Tests set it.
	Ask func(prompt string, errOut io.Writer) (string, error)
}

// maxDraft is the longest description DescribeFolder keeps.
const maxDraft = 300

// GenerateDescription drafts a project's description with the AI command,
// prints it, and keeps it on a yes (or with Yes).
//
// The answer is only ever text: it is checked, then handed to DescribeFolder
// like a description the owner typed.
func GenerateDescription(shortName string, o GenerateOptions) error {
	folder, err := FindFolder(shortName)
	if err != nil {
		return err
	}

	prompt, err := descriptionPrompt(shortName, folder.Path)
	if err != nil {
		return err
	}
	if o.ShowPrompt {
		_, err := fmt.Fprint(o.Out, prompt)
		return err
	}

	ask := o.Ask
	if ask == nil {
		ask = aiutil.Ask
	}
	answer, err := ask(prompt, o.Err)
	if err != nil {
		return err
	}
	draft := strings.TrimSpace(answer)
	draft = strings.TrimSpace(strings.TrimSuffix(strings.Trim(draft, "\"'`"), "."))
	if draft == "" || len(draft) > maxDraft || strings.ContainsAny(draft, "\r\n") {
		return fmt.Errorf("the AI did not answer with one line under %d bytes: %q", maxDraft, answer)
	}

	if o.Yes {
		return DescribeFolder(o.Out, shortName, draft, false)
	}
	if _, err := fmt.Fprintln(o.Out, draft); err != nil {
		return err
	}
	// Without a person to ask, the draft is only shown.
	if keep, err := cli.Confirm(o.In, o.Err, "Keep this description? [y/N] "); err != nil || !keep {
		return err
	}
	return DescribeFolder(o.Out, shortName, draft, false)
}

// descriptionPrompt is everything the AI is told about the project: its
// name, the top-level paths it may be told of and the start of its README.
func descriptionPrompt(shortName, path string) (string, error) {
	tree, err := aiutil.Tree(path, 200, 1)
	if err != nil {
		return "", needsGit(err)
	}
	readmes, err := aiutil.Files(path, "README.md", "README", "readme.md")
	if err != nil {
		return "", needsGit(err)
	}

	var b strings.Builder
	b.WriteString("Write a description of this software project: one line, under 120 characters, saying what this project is and for whom; no quotes, no trailing period. Answer with that line only.\n\n")
	fmt.Fprintf(&b, "Project: %s\n\nFiles:\n%s\n", shortName, strings.Join(tree, "\n"))
	if len(readmes) > 0 {
		fmt.Fprintf(&b, "\n%s:\n%s\n", readmes[0].Path, readmes[0].Content)
	}
	return b.String(), nil
}

// needsGit names git as the cause only when git is what is missing.
func needsGit(err error) error {
	if _, lookErr := exec.LookPath("git"); lookErr != nil {
		return fmt.Errorf("--generate needs git to tell which files may be sent: %w", err)
	}
	return err
}
