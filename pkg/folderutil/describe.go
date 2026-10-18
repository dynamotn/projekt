package folderutil

import (
	"fmt"
	"io"
	"strings"

	"gitlab.com/dynamo-tools/projekt/pkg/lazypath"
)

// DescribeFolder sets, clears or prints the one-line description of a
// project.
//
// With no text and no clear, it prints the description the project has, and
// exits non-zero through the error when it has none, so a script can ask. A
// project found inside a workspace has no entry of its own: like its tags, it
// carries the workspace's description, and the workspace is what to describe.
func DescribeFolder(out io.Writer, shortName string, text string, clear bool) error {
	folder, err := FindFolder(shortName)
	if err != nil {
		return err
	}

	text = strings.TrimSpace(text)
	if text == "" && !clear {
		if folder.Description == "" {
			return fmt.Errorf("%s has no description", shortName)
		}
		_, err := fmt.Fprintln(out, folder.Description)
		return err
	}
	if strings.ContainsAny(text, "\n\r\t") {
		return fmt.Errorf("a description is one line")
	}

	if err := lazypath.LoadError(); err != nil {
		return err
	}
	if _, _, isWorktree := lazypath.SplitWorktreeName(shortName); isWorktree {
		return fmt.Errorf("%s is a worktree; it carries the description of the project it belongs to", shortName)
	}
	if _, _, isOwn := lazypath.FindOwnFolder(folder.Path); !isOwn {
		return fmt.Errorf("%s has no entry of its own: it is found inside a workspace, and carries the workspace's description", shortName)
	}

	if clear {
		text = ""
	}
	if err := lazypath.SetFolderDescription(folder.Path, text); err != nil {
		return err
	}

	if text == "" {
		_, err := fmt.Fprintf(out, "%s has no description now\n", shortName)
		return err
	}
	_, err = fmt.Fprintf(out, "%s: %s\n", shortName, text)
	return err
}
