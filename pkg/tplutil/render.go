package tplutil

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"gitlab.com/dynamo.foss/projekt/pkg/cli"
)

// RenderOptions drives one `t new` run.
type RenderOptions struct {
	// Template is the store entry to render.
	Template Template
	// Dest is where the result is written: a file path or a folder, depending
	// on the template kind. Empty means the current folder.
	Dest string
	// Name overrides the rendered file name and the `.Name` variable.
	Name string
	// Values are the user supplied variables, reachable through `.Values`.
	Values Values
	// Force allows overwriting files that already exist.
	Force bool
	// DryRun renders to Out instead of touching the filesystem.
	DryRun bool
	// Out receives the rendered content of a dry run.
	Out io.Writer
	// In is where the prompt functions read their answers from.
	In io.Reader
	// Prompt receives the questions a template asks while it renders. Nil
	// means standard error, so a piped dry run still gets only the template.
	Prompt io.Writer
	// Interactive lets the prompt functions ask. Without it they take their
	// default, and a question without one is an error.
	Interactive bool
}

// BaseContext returns what a template is given before any value is asked for,
// so that a default like `{{ .User }}` in a manifest renders the same way the
// template itself would.
func BaseContext(o RenderOptions) (map[string]any, error) {
	values, err := WithData(o)
	if err != nil {
		return nil, err
	}
	o.Values = values

	if o.Template.IsDir() {
		return context(o, ""), nil
	}
	target, err := fileTarget(o)
	if err != nil {
		return nil, err
	}
	return context(o, target), nil
}

// Destination returns the folder a render writes into, which is where a
// template's `after` commands run.
func Destination(o RenderOptions) (string, error) {
	if !o.Template.IsDir() {
		target, err := fileTarget(o)
		if err != nil {
			return "", err
		}
		return filepath.Dir(target), nil
	}
	dest := o.Dest
	if dest == "" {
		dest = "."
	}
	return filepath.Abs(dest)
}

// Render renders a template and reports every file it created, in the order
// they were written.
func Render(o RenderOptions) ([]string, error) {
	if o.Out == nil {
		o.Out = os.Stdout
	}
	if o.Values == nil {
		o.Values = Values{}
	}

	// A data file is a default: it fills what nobody asked about, and loses to
	// anything given on the command line.
	values, err := WithData(o)
	if err != nil {
		return nil, err
	}
	o.Values = values

	engine, err := newEngine(o)
	if err != nil {
		return nil, err
	}

	if o.Template.IsDir() {
		return renderDir(engine, o)
	}
	return renderFile(engine, o)
}

// renderFile renders a single file template.
func renderFile(e *engine, o RenderOptions) ([]string, error) {
	target, err := fileTarget(o)
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(o.Template.Path)
	if err != nil {
		return nil, fmt.Errorf("cannot read template %s: %w", o.Template.Path, err)
	}

	rendered, err := e.execute(o.Template.Name, string(data), context(o, target))
	if err != nil {
		return nil, err
	}

	if o.DryRun {
		_, err = o.Out.Write(rendered)
		return []string{target}, err
	}
	if err := writeFile(target, rendered, o.Force, Attributes{}); err != nil {
		return nil, err
	}
	return []string{target}, nil
}

// fileTarget works out which file a single-file template writes to.
//
// The destination may be left out (current folder), name a folder (the
// template file name is kept) or name the file itself.
func fileTarget(o RenderOptions) (string, error) {
	// The template file name without its .tmpl suffix is the natural default.
	base := strings.TrimSuffix(filepath.Base(o.Template.Path), TemplateExt)
	if o.Name != "" {
		base = o.Name
	}

	dest := o.Dest
	if dest == "" {
		return filepath.Abs(base)
	}

	// A destination that exists as a folder, or ends with a separator, is where
	// the file goes; anything else is the file itself.
	if info, err := os.Stat(dest); err == nil && info.IsDir() {
		return filepath.Abs(filepath.Join(dest, base))
	} else if err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("cannot access %s: %w", dest, err)
	}
	if strings.HasSuffix(dest, string(os.PathSeparator)) {
		return filepath.Abs(filepath.Join(dest, base))
	}
	return filepath.Abs(dest)
}

// Rendered is one file a folder template produces, before anything is written.
//
// Collecting the whole tree first is what lets `t new`, `--dry-run`, `t diff`
// and `t apply` be the same render seen four ways.
type Rendered struct {
	// Rel is the path relative to the destination, with "/" whatever the
	// platform is.
	Rel string
	// Content is the rendered file, or the target of a symbolic link.
	Content []byte
	// Attrs are what the name asked the file to be.
	Attrs Attributes
	// IsDir reports whether this is a folder rather than a file.
	IsDir bool
}

// Collect renders a folder template into memory, without touching the disk.
func Collect(o RenderOptions) ([]Rendered, error) {
	if !o.Template.IsDir() {
		return nil, fmt.Errorf("%s is a file template, there is no tree to collect", o.Template.Name)
	}
	if o.Values == nil {
		o.Values = Values{}
	}
	values, err := WithData(o)
	if err != nil {
		return nil, err
	}
	o.Values = values

	engine, err := newEngine(o)
	if err != nil {
		return nil, err
	}
	return collectDir(engine, o)
}

// collectDir walks the template and renders every name and every file.
func collectDir(e *engine, o RenderOptions) ([]Rendered, error) {
	root, err := destinationRoot(o)
	if err != nil {
		return nil, err
	}

	ignore, err := e.loadIgnore(o, context(o, ""))
	if err != nil {
		return nil, err
	}

	var files []Rendered
	err = filepath.WalkDir(o.Template.Path, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(o.Template.Path, path)
		if err != nil {
			return err
		}
		if relative == "." {
			return nil
		}
		if skip, dir := isReserved(entry, relative); skip {
			if dir {
				return fs.SkipDir
			}
			return nil
		}

		rendered, attrs, err := renderRelative(e, o, relative)
		if err != nil {
			return err
		}
		// The ignore file names what the template writes, not what it is made
		// of, so it is matched against the rendered path.
		if ignore.Match(rendered, entry.IsDir()) {
			cli.Debug("Ignored %s", rendered)
			if entry.IsDir() {
				return fs.SkipDir
			}
			return nil
		}

		if entry.IsDir() {
			files = append(files, Rendered{Rel: rendered, Attrs: attrs, IsDir: true})
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("cannot read template file %s: %w", path, err)
		}
		target := filepath.Join(root, filepath.FromSlash(rendered))
		content, err := e.execute(o.Template.Name+"/"+relative, string(data), context(o, target))
		if err != nil {
			return err
		}
		files = append(files, Rendered{Rel: rendered, Content: content, Attrs: attrs})
		return nil
	})
	if err != nil {
		return nil, err
	}
	return files, nil
}

// destinationRoot resolves where a folder template writes.
func destinationRoot(o RenderOptions) (string, error) {
	root := o.Dest
	if root == "" {
		root = "."
	}
	root, err := filepath.Abs(root)
	if err != nil {
		return "", fmt.Errorf("cannot resolve destination %s: %w", o.Dest, err)
	}
	return root, nil
}

// renderDir renders a folder template: every file is rendered, and so is every
// path segment, so `{{ .Name }}/main.go.tmpl` lands under the project name.
func renderDir(e *engine, o RenderOptions) ([]string, error) {
	root, err := destinationRoot(o)
	if err != nil {
		return nil, err
	}
	files, err := collectDir(e, o)
	if err != nil {
		return nil, err
	}

	record := RenderRecord{
		Template:   o.Template.Name,
		Name:       nameOf(o),
		RenderedAt: time.Now().UTC(),
		Values:     o.Values,
		Files:      map[string]string{},
	}

	var written []string
	for _, file := range files {
		target := filepath.Join(root, filepath.FromSlash(file.Rel))

		if file.IsDir {
			if o.DryRun {
				continue
			}
			if err := os.MkdirAll(target, file.Attrs.DirMode()); err != nil {
				return nil, err
			}
			continue
		}

		if o.DryRun {
			if err := describe(o.Out, target, file.Content, file.Attrs); err != nil {
				return nil, err
			}
			written = append(written, target)
			continue
		}

		if err := writeFile(target, file.Content, o.Force, file.Attrs); err != nil {
			return nil, err
		}
		record.Files[file.Rel] = hashOf(file.Content)
		written = append(written, target)
	}

	if o.DryRun {
		return written, nil
	}
	// Remembering what was written, and what it was rendered from, is what
	// makes `t diff` and `t apply` possible later.
	if err := recordRender(root, record); err != nil {
		return nil, err
	}
	return written, nil
}

// nameOf is the `.Name` a render used, for the record to replay it.
func nameOf(o RenderOptions) string {
	if name, ok := context(o, "")["Name"].(string); ok {
		return name
	}
	return o.Name
}

// isReserved reports whether an entry of a folder template describes the
// template rather than belonging to what it creates, and whether skipping it
// means skipping a whole folder.
func isReserved(entry fs.DirEntry, relative string) (skip, dir bool) {
	name := entry.Name()
	if entry.IsDir() {
		// Repository metadata, and the folder of shared pieces, which are
		// rendered by the files that call them rather than on their own.
		return name == ".git" || name == PartialsDir, true
	}
	// The manifest, the data file and the ignore list describe the template;
	// they are not part of the output.
	return name == VarsFile || name == IgnoreFile || isDataFile(name), false
}

// describe prints one file of a dry run, saying what it would be when the
// name asked for more than plain content.
func describe(out io.Writer, target string, content []byte, attrs Attributes) error {
	header := "# " + target
	switch {
	case attrs.Symlink:
		header += " -> " + strings.TrimSpace(string(content))
	case attrs.Any():
		header += fmt.Sprintf(" (%s)", attrs.FileMode())
	}
	if _, err := fmt.Fprintln(out, header); err != nil {
		return err
	}
	if attrs.Symlink {
		return nil
	}
	if _, err := out.Write(content); err != nil {
		return err
	}
	if len(content) > 0 && !bytes.HasSuffix(content, []byte("\n")) {
		if _, err := fmt.Fprintln(out); err != nil {
			return err
		}
	}
	return nil
}

// renderRelative renders the path segments of a folder template, reads the
// attribute prefixes off them and strips the .tmpl suffix from the last one.
//
// The prefixes are read before the segment is rendered, so a value can never
// turn a file into an executable or a symbolic link.
func renderRelative(e *engine, o RenderOptions, relative string) (string, Attributes, error) {
	segments := strings.Split(relative, string(os.PathSeparator))
	var attrs Attributes

	for i, segment := range segments {
		bare, segmentAttrs := ParseAttributes(segment)
		if i == len(segments)-1 {
			attrs = segmentAttrs
		}

		rendered, err := e.execute("path:"+relative, bare, context(o, ""))
		if err != nil {
			return "", attrs, err
		}
		name := strings.TrimSpace(string(rendered))
		if name == "" {
			return "", attrs, fmt.Errorf("path segment %q of %s renders to an empty name", segment, relative)
		}
		if i == len(segments)-1 {
			name = strings.TrimSuffix(name, TemplateExt)
		}
		// A rendered segment must stay one single folder level: a value
		// containing "/" or ".." would escape the destination folder.
		if name != filepath.Base(name) || name == ".." {
			return "", attrs, fmt.Errorf("path segment %q of %s renders to an invalid name %q", segment, relative, name)
		}
		segments[i] = name
	}
	return strings.Join(segments, "/"), attrs, nil
}

// context builds the data a template is executed with.
func context(o RenderOptions, target string) map[string]any {
	name := o.Name
	if name == "" && target != "" {
		// The output file name without its extension is the most useful
		// default: `t new go-main cmd/serve.go` gives `.Name` "serve".
		base := filepath.Base(target)
		name = strings.TrimSuffix(base, filepath.Ext(base))
	}

	dir := o.Dest
	if dir == "" {
		dir, _ = os.Getwd()
	}
	if abs, err := filepath.Abs(dir); err == nil {
		dir = abs
	}
	if target != "" && !o.Template.IsDir() {
		dir = filepath.Dir(target)
	}

	if name == "" && o.Template.IsDir() {
		// A folder template creates the project, so the folder it is written
		// to is its name: `t new go-cli ./myapp` gives `.Name` "myapp".
		name = filepath.Base(dir)
	}

	project := filepath.Base(dir)
	if o.Template.IsDir() && name != "" {
		project = name
	}

	store, _ := Dir()
	home, _ := os.UserHomeDir()
	hostname, _ := os.Hostname()

	now := time.Now()
	return map[string]any{
		"Name":     name,
		"Project":  project,
		"Dir":      dir,
		"Path":     target,
		"Template": o.Template.Name,
		"Source":   templateSource(o.Template),
		"Store":    store,
		"User":     currentUser(),
		"Home":     home,
		"Hostname": hostname,
		"OS":       runtime.GOOS,
		"Arch":     runtime.GOARCH,
		"Env":      environment(),
		"Now":      now,
		"Date":     now.Format("2006-01-02"),
		"Year":     now.Format("2006"),
		"Values":   map[string]any(o.Values),
	}
}

// environment is the process environment as a map, so a template can read
// `{{ .Env.EDITOR }}` without shelling out.
func environment() map[string]string {
	env := map[string]string{}
	for _, entry := range os.Environ() {
		key, value, found := strings.Cut(entry, "=")
		if found {
			env[key] = value
		}
	}
	return env
}

func currentUser() string {
	if u, err := user.Current(); err == nil && u.Username != "" {
		return u.Username
	}
	return os.Getenv("USER")
}

// writeFile writes the rendered content, refusing to clobber an existing file
// unless --force was given.
func writeFile(target string, content []byte, force bool, attrs Attributes) error {
	if !force {
		if _, err := os.Lstat(target); err == nil {
			return fmt.Errorf("%s already exists, use --force to overwrite it", target)
		} else if !os.IsNotExist(err) {
			return fmt.Errorf("cannot access %s: %w", target, err)
		}
	}

	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return fmt.Errorf("cannot create folder %s: %w", filepath.Dir(target), err)
	}

	if attrs.Symlink {
		return writeSymlink(target, string(content))
	}

	// The file may be there from an earlier run, read-only, or of another
	// mode: replacing it is what --force asked for.
	if err := os.Remove(target); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("cannot replace %s: %w", target, err)
	}
	if err := os.WriteFile(target, content, attrs.FileMode()); err != nil {
		return fmt.Errorf("cannot write %s: %w", target, err)
	}
	// WriteFile obeys the umask, which would drop the bits the name asked for.
	if attrs.Any() {
		if err := os.Chmod(target, attrs.FileMode()); err != nil {
			return fmt.Errorf("cannot set the mode of %s: %w", target, err)
		}
	}
	cli.Debug("Rendered %s", target)
	return nil
}

// writeSymlink points a name at what the template rendered.
func writeSymlink(target, link string) error {
	link = strings.TrimSpace(link)
	if link == "" {
		return fmt.Errorf("%s is a symbolic link whose target renders empty", target)
	}
	if err := os.Remove(target); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("cannot replace %s: %w", target, err)
	}
	if err := os.Symlink(link, target); err != nil {
		return fmt.Errorf("cannot link %s: %w", target, err)
	}
	cli.Debug("Linked %s -> %s", target, link)
	return nil
}

// RenderString runs one piece of text through the engine, with the same
// functions and the same missing-value behaviour as a template file.
//
// It is what a command line in a recipe goes through, so that `{{ .Name }}`
// means there what it means everywhere else.
func RenderString(name, text string, data map[string]any) (string, error) {
	rendered, err := execute(name, text, data)
	if err != nil {
		return "", err
	}
	return string(rendered), nil
}
