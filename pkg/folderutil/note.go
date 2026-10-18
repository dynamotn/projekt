package folderutil

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"gitlab.com/dynamo-tools/projekt/pkg/cli"
)

// NoteFile is where a project keeps its note, relative to the project. It sits
// beside the template record in .projekt, which is the project's own folder for
// what projekt knows about it, and moves and archives with the project.
const NoteFile = ".projekt/notes.md"

// ErrNoNote is returned when a project has no note to show.
var ErrNoNote = errors.New("no note")

// NoteOptions drives `projekt folder note`.
type NoteOptions struct {
	// Add appends this text as a dated entry.
	Add string
	// Edit opens the note in the editor.
	Edit bool
	// Now is the clock a new entry is dated with; nil means time.Now.
	Now func() time.Time
	// In, Out and Err are the editor's streams.
	In       io.Reader
	Out, Err io.Writer
}

// NoteFolder prints, extends or edits the note of a project.
//
// An empty short name means the project the current folder is in, which is
// the common case: `folder note -a "left the migration half done"` on the way
// out of the door.
func NoteFolder(out io.Writer, shortName string, o NoteOptions) error {
	folder, err := noteTarget(shortName)
	if err != nil {
		return err
	}
	path := filepath.Join(folder.Path, NoteFile)

	if text := strings.TrimSpace(o.Add); text != "" {
		if err := appendNote(path, text, o.Now); err != nil {
			return err
		}
		if o.Edit {
			return editNote(path, o)
		}
		_, err := fmt.Fprintf(out, "Noted in %s\n", folder.ShortName)
		return err
	}
	if o.Edit {
		return editNote(path, o)
	}

	content, err := os.ReadFile(path)
	if os.IsNotExist(err) || (err == nil && strings.TrimSpace(string(content)) == "") {
		return fmt.Errorf("%w for %s; add one with --add or --edit", ErrNoNote, folder.ShortName)
	}
	if err != nil {
		return err
	}
	_, err = out.Write(content)
	return err
}

// noteTarget resolves which project a note belongs to.
func noteTarget(shortName string) (ParsedFolder, error) {
	if strings.TrimSpace(shortName) != "" {
		return FindFolder(shortName)
	}
	cwd, err := os.Getwd()
	if err != nil {
		return ParsedFolder{}, fmt.Errorf("cannot resolve the current folder: %w", err)
	}
	folder, err := CurrentProject(cwd)
	if err != nil {
		return ParsedFolder{}, fmt.Errorf("%w; name the project", err)
	}
	return folder, nil
}

// appendNote adds a dated entry to the note, creating it when needed.
func appendNote(path string, text string, now func() time.Time) error {
	if now == nil {
		now = time.Now
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}

	existing, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	var entry strings.Builder
	if len(existing) > 0 && !strings.HasSuffix(string(existing), "\n") {
		entry.WriteString("\n")
	}
	fmt.Fprintf(&entry, "- %s: %s\n", now().Format("2006-01-02"), text)

	file, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	if _, err := file.WriteString(entry.String()); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}

// editNote opens the note in the editor, with the project as the working
// directory so that a relative link in it resolves.
func editNote(path string, o NoteOptions) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	editor := cli.EditorCommand(os.Getenv)
	cmd := exec.Command(editor[0], append(append([]string{}, editor[1:]...), path)...)
	cmd.Dir = filepath.Dir(filepath.Dir(path))
	cmd.Stdin = o.In
	cmd.Stdout = o.Out
	cmd.Stderr = o.Err
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("editor %s failed: %w", editor[0], err)
	}
	return nil
}

// NoteHeadline returns the first line of substance in a project's note, or
// "" when there is none: a heading loses its marks and a list item its bullet,
// so that a column of them reads as prose.
func NoteHeadline(projectPath string) string {
	content, err := os.ReadFile(filepath.Join(projectPath, NoteFile))
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(content), "\n") {
		line = strings.TrimSpace(line)
		line = strings.TrimLeft(line, "#")
		line = strings.TrimPrefix(strings.TrimSpace(line), "- ")
		line = strings.TrimPrefix(line, "* ")
		if line = strings.TrimSpace(line); line != "" {
			return line
		}
	}
	return ""
}
