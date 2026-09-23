package tplutil

import (
	"bytes"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/user"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/Masterminds/sprig"

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
}

// BaseContext returns what a template is given before any value is asked for,
// so that a default like `{{ .User }}` in a manifest renders the same way the
// template itself would.
func BaseContext(o RenderOptions) (map[string]any, error) {
	if o.Template.IsDir() {
		return context(o, ""), nil
	}
	target, err := fileTarget(o)
	if err != nil {
		return nil, err
	}
	return context(o, target), nil
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
	if o.Template.IsDir() {
		return renderDir(o)
	}
	return renderFile(o)
}

// renderFile renders a single file template.
func renderFile(o RenderOptions) ([]string, error) {
	target, err := fileTarget(o)
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(o.Template.Path)
	if err != nil {
		return nil, fmt.Errorf("cannot read template %s: %w", o.Template.Path, err)
	}

	rendered, err := execute(o.Template.Name, string(data), context(o, target))
	if err != nil {
		return nil, err
	}

	if o.DryRun {
		_, err = o.Out.Write(rendered)
		return []string{target}, err
	}
	if err := writeFile(target, rendered, o.Force); err != nil {
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

// renderDir renders a folder template: every file is rendered, and so is every
// path segment, so `{{ .Name }}/main.go.tmpl` lands under the project name.
func renderDir(o RenderOptions) ([]string, error) {
	root := o.Dest
	if root == "" {
		root = "."
	}
	root, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("cannot resolve destination %s: %w", o.Dest, err)
	}

	var written []string
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
		// A dotfile is part of what a project needs — .gitignore, .github,
		// .env.example — so only repository metadata is left out.
		if entry.IsDir() && entry.Name() == ".git" {
			return fs.SkipDir
		}
		// The manifest describes the template, it is not part of the output.
		if !entry.IsDir() && entry.Name() == VarsFile {
			return nil
		}

		target, err := renderPath(o, root, relative)
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if o.DryRun {
				return nil
			}
			return os.MkdirAll(target, 0o755)
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("cannot read template file %s: %w", path, err)
		}
		rendered, err := execute(o.Template.Name+"/"+relative, string(data), context(o, target))
		if err != nil {
			return err
		}

		if o.DryRun {
			if _, err := fmt.Fprintf(o.Out, "# %s\n", target); err != nil {
				return err
			}
			if _, err := o.Out.Write(rendered); err != nil {
				return err
			}
			if len(rendered) > 0 && !bytes.HasSuffix(rendered, []byte("\n")) {
				if _, err := fmt.Fprintln(o.Out); err != nil {
					return err
				}
			}
			written = append(written, target)
			return nil
		}

		if err := writeFile(target, rendered, o.Force); err != nil {
			return err
		}
		written = append(written, target)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return written, nil
}

// renderPath renders the path segments of a folder template and strips the
// .tmpl suffix from the last one.
func renderPath(o RenderOptions, root, relative string) (string, error) {
	segments := strings.Split(relative, string(os.PathSeparator))
	for i, segment := range segments {
		rendered, err := execute("path:"+relative, segment, context(o, ""))
		if err != nil {
			return "", err
		}
		segment = strings.TrimSpace(string(rendered))
		if segment == "" {
			return "", fmt.Errorf("path segment %q of %s renders to an empty name", segments[i], relative)
		}
		if i == len(segments)-1 {
			segment = strings.TrimSuffix(segment, TemplateExt)
		}
		// A rendered segment must stay one single folder level: a value
		// containing "/" or ".." would escape the destination folder.
		if segment != filepath.Base(segment) || segment == ".." {
			return "", fmt.Errorf("path segment %q of %s renders to an invalid name %q", segments[i], relative, segment)
		}
		segments[i] = segment
	}
	return filepath.Join(append([]string{root}, segments...)...), nil
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

	now := time.Now()
	return map[string]any{
		"Name":     name,
		"Project":  project,
		"Dir":      dir,
		"Path":     target,
		"Template": o.Template.Name,
		"User":     currentUser(),
		"Now":      now,
		"Date":     now.Format("2006-01-02"),
		"Year":     now.Format("2006"),
		"Values":   map[string]any(o.Values),
	}
}

func currentUser() string {
	if u, err := user.Current(); err == nil && u.Username != "" {
		return u.Username
	}
	return os.Getenv("USER")
}

// execute parses and runs one template with the sprig function set.
func execute(name, text string, data map[string]any) ([]byte, error) {
	// missingkey=zero keeps `{{ .Values.foo | default "bar" }}` working for
	// values the user did not set, instead of failing the whole render.
	t, err := template.New(name).Funcs(sprig.TxtFuncMap()).Option("missingkey=zero").Parse(text)
	if err != nil {
		return nil, fmt.Errorf("cannot parse template %s: %w", name, err)
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("cannot render template %s: %w", name, err)
	}
	return buf.Bytes(), nil
}

// writeFile writes the rendered content, refusing to clobber an existing file
// unless --force was given.
func writeFile(target string, content []byte, force bool) error {
	if !force {
		if _, err := os.Stat(target); err == nil {
			return fmt.Errorf("%s already exists, use --force to overwrite it", target)
		} else if !os.IsNotExist(err) {
			return fmt.Errorf("cannot access %s: %w", target, err)
		}
	}

	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return fmt.Errorf("cannot create folder %s: %w", filepath.Dir(target), err)
	}
	if err := os.WriteFile(target, content, 0o644); err != nil {
		return fmt.Errorf("cannot write %s: %w", target, err)
	}
	cli.Debug("Rendered %s", target)
	return nil
}
