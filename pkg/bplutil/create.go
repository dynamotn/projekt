package bplutil

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gitlab.com/dynamo-tools/projekt/pkg/cli"
	"gitlab.com/dynamo-tools/projekt/pkg/folderutil"
	"gitlab.com/dynamo-tools/projekt/pkg/lazypath"
	"gitlab.com/dynamo-tools/projekt/pkg/tplutil"
)

// CreateOptions drives one `b new` run.
type CreateOptions struct {
	// Recipe is the store entry to create from.
	Recipe Recipe
	// Target is what was asked for: a project name, or a path when it has a
	// separator in it. A name is created inside the workspace.
	Target string
	// Workspace overrides the recipe's, and is ignored when Target is a path.
	Workspace string
	// Values are the variables the template is rendered with.
	Values tplutil.Values
	// Force allows overwriting files that already exist.
	Force bool
	// DryRun works out the whole plan without touching anything.
	DryRun bool
	// NoRegister leaves the configuration alone.
	NoRegister bool
	// Out receives the rendered content of a dry run, which is a lot of text
	// and is usually thrown away.
	Out io.Writer
	// Log receives what is being done, or would be: the clone, each command,
	// the remote. A dry run is worth nothing if it goes somewhere unread.
	Log io.Writer
	// NoHooks skips the recipe's `after` commands.
	NoHooks bool
	// Interactive lets the template's prompt functions ask while it renders.
	Interactive bool
	// In, HookOut and HookErr are the streams a hook runs with. A hook is a
	// command someone will want to watch, so these are the terminal.
	In               io.Reader
	HookOut, HookErr io.Writer
}

// Plan is what `b new` is about to do, worked out before anything happens.
type Plan struct {
	// Recipe and Template are what the files come from.
	Recipe   Recipe
	Template tplutil.Template
	// Path is the absolute folder the project is created in.
	Path string
	// Name is the project name, which is also the template's `.Name`.
	Name string
	// Register reports whether a folder entry will be added.
	Register bool
	// CoveredBy names the workspace that already reaches the project, when
	// one does. No entry is added then: the workspace already sees it.
	CoveredBy string
	// coveredPrefix is that workspace's prefix, which the short name carries.
	coveredPrefix string
	// Reason says why nothing is registered, for the plan to print.
	Reason string
}

// Result is what `b new` did.
type Result struct {
	Plan
	// Files are the files created, in the order they were written.
	Files []string
	// Registered reports whether the configuration was changed.
	Registered bool
}

// Resolve works out what a create would do, without doing any of it.
func Resolve(o CreateOptions) (Plan, error) {
	if err := o.Recipe.Validate(); err != nil {
		return Plan{}, err
	}

	var tpl tplutil.Template
	if o.Recipe.Source.Template != "" {
		found, err := tplutil.Get(o.Recipe.Source.Template)
		if err != nil {
			return Plan{}, fmt.Errorf("boilerplate %q: %w", o.Recipe.Name, err)
		}
		tpl = found
	} else {
		// A clone is a folder of files like any other, so the rest of the plan
		// — the name, the destination, the context a default renders with —
		// works out the same way.
		tpl = tplutil.Template{Name: o.Recipe.Name, Kind: tplutil.KindDir}
	}

	path, name, err := destination(o)
	if err != nil {
		return Plan{}, err
	}

	plan := Plan{Recipe: o.Recipe, Template: tpl, Path: path, Name: name}
	plan.Register, plan.CoveredBy, plan.coveredPrefix, plan.Reason = registration(o, path)
	return plan, nil
}

// destination works out where the project goes and what it is called.
//
// A target with a separator in it is a path, and is used as it is. A plain
// name is created inside the workspace, which is what makes `pj <name>` work
// right after.
func destination(o CreateOptions) (string, string, error) {
	target := strings.TrimSpace(o.Target)
	if target == "" {
		return "", "", fmt.Errorf("no project name given")
	}

	if isPath(target) {
		path, err := lazypath.NormalizePath(target)
		if err != nil {
			return "", "", err
		}
		return path, filepath.Base(path), nil
	}
	if target != filepath.Base(target) || target == "." || target == ".." {
		return "", "", fmt.Errorf("invalid project name %q", target)
	}

	workspace := o.Workspace
	if workspace == "" {
		workspace = o.Recipe.Register.Workspace
	}
	if workspace == "" {
		// No workspace anywhere: the current folder is the only sensible
		// place left, and the project still gets its own folder in it.
		cwd, err := os.Getwd()
		if err != nil {
			return "", "", fmt.Errorf("cannot resolve the current folder: %w", err)
		}
		workspace = cwd
	}

	base, err := lazypath.NormalizePath(workspace)
	if err != nil {
		return "", "", err
	}
	return filepath.Join(base, target), target, nil
}

// isPath reports whether a target names a location rather than a project.
func isPath(target string) bool {
	return strings.ContainsRune(target, filepath.Separator) ||
		strings.HasPrefix(target, "~") ||
		target == "." || target == ".."
}

// registration decides whether the new project needs an entry of its own.
func registration(o CreateOptions, path string) (register bool, coveredBy, coveredPrefix, reason string) {
	if o.NoRegister {
		return false, "", "", "--no-register"
	}
	if o.Recipe.Register.Skip {
		return false, "", "", "the recipe sets register.skip"
	}
	if err := lazypath.LoadError(); err != nil {
		return false, "", "", "the configuration cannot be read"
	}
	if exists, _ := lazypath.CheckFolderExist(path); exists {
		return false, "", "", "it is already in the configuration"
	}
	if workspace, ok := folderutil.CoveringWorkspace(path); ok {
		// A workspace already turns every child into a project, so an entry
		// of its own would only be a second name for the same folder.
		return false, workspace.Path, workspace.Prefix, ""
	}
	return true, "", "", ""
}

// Vars returns what the recipe asks for: its own list, or what the template
// it renders would ask for.
func (p Plan) Vars() ([]tplutil.Var, error) {
	if len(p.Recipe.Vars) > 0 {
		return p.Recipe.Vars, nil
	}
	return tplutil.Vars(p.Template)
}

// BaseContext returns what a default is rendered with, so that a recipe can
// say `default: "example.com/{{ .Name }}"`.
func (p Plan) BaseContext() (map[string]any, error) {
	return tplutil.BaseContext(p.renderOptions(CreateOptions{}, nil, nil))
}

// Values returns what the template is rendered with: the data files of the
// store and the template, under everything the recipe was given.
func (p Plan) Values(values tplutil.Values) (tplutil.Values, error) {
	return tplutil.WithData(p.renderOptions(CreateOptions{Values: values}, values, nil))
}

func (p Plan) renderOptions(o CreateOptions, values tplutil.Values, out io.Writer) tplutil.RenderOptions {
	return tplutil.RenderOptions{
		Template:    p.Template,
		Dest:        p.Path,
		Name:        p.Name,
		Values:      values,
		Force:       o.Force,
		DryRun:      o.DryRun,
		Out:         out,
		In:          o.In,
		Prompt:      o.HookErr,
		Interactive: o.Interactive,
	}
}

// Create renders the project and registers it.
//
// The configuration is only written once the files are there: a folder entry
// pointing at a project that failed to be created would be worse than none.
func Create(o CreateOptions) (Result, error) {
	plan, err := Resolve(o)
	if err != nil {
		return Result{}, err
	}

	if !o.Force && !o.DryRun {
		if err := checkEmpty(plan.Path); err != nil {
			return Result{}, err
		}
	}

	out := o.Out
	if out == nil {
		out = io.Discard
	}
	log := o.Log
	if log == nil {
		log = io.Discard
	}

	var files []string
	if plan.Recipe.Source.Repo != "" {
		files, err = createFromRepo(plan, o, log)
	} else {
		files, err = tplutil.Render(plan.renderOptions(o, o.Values, out))
	}
	if err != nil {
		return Result{}, err
	}

	result := Result{Plan: plan, Files: files}

	// The template's own commands belong to the template, and run before the
	// recipe's: `go mod tidy` is part of rendering a Go project, whatever the
	// recipe goes on to do with it.
	if plan.Recipe.Source.Repo == "" && !o.NoHooks {
		if err := runTemplateHooks(log, plan, o); err != nil {
			return result, err
		}
	}

	if !o.DryRun || len(plan.Recipe.After) > 0 || plan.Recipe.Register.Remote != nil {
		if err := finish(log, plan, o); err != nil {
			return result, err
		}
	}
	// The recipe brings questions and commands of its own; remembering only
	// the template it rendered would lose both when the project is next
	// brought up to date.
	if !o.DryRun && plan.Recipe.Source.Template != "" {
		record := tplutil.RecipeRecord{
			Recipe:    plan.Recipe.Name,
			Template:  plan.Template.Name,
			Name:      plan.Name,
			CreatedAt: time.Now().UTC(),
			Values:    o.Values,
		}
		if err := tplutil.RecordRecipe(plan.Path, record); err != nil {
			return result, err
		}
	}

	if o.DryRun || !plan.Register {
		return result, nil
	}

	folder := &lazypath.Folder{
		Path:     plan.Path,
		Prefix:   plan.Recipe.Register.Prefix,
		Tags:     plan.Recipe.Register.Tags,
		Priority: plan.Recipe.Register.Priority,
	}
	if err := folder.AddToConfig(); err != nil {
		// The project exists; say so rather than pretending the whole thing
		// failed, and let the caller report the configuration problem.
		return result, fmt.Errorf("created %s but could not register it: %w", plan.Path, err)
	}
	result.Registered = true
	return result, nil
}

// runTemplateHooks runs what the template's own manifest asks for, in the
// project that was just created.
func runTemplateHooks(log io.Writer, plan Plan, o CreateOptions) error {
	options := plan.renderOptions(o, o.Values, nil)
	base, err := tplutil.BaseContext(options)
	if err != nil {
		return err
	}

	return tplutil.RunAfter(tplutil.HookOptions{
		Template: plan.Template,
		Dir:      plan.Path,
		Context:  base,
		DryRun:   o.DryRun,
		Log:      log,
		In:       o.In,
		Out:      o.HookOut,
		Err:      o.HookErr,
	})
}

// checkEmpty refuses to create a project on top of one that is already there.
func checkEmpty(path string) error {
	entries, err := os.ReadDir(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("cannot access %s: %w", path, err)
	}
	if len(entries) > 0 {
		return fmt.Errorf("%s is not empty, use --force to create into it anyway", path)
	}
	return nil
}

// ShortName is the name `pj` reaches the project by: through the entry that is
// about to be added, or through the workspace that already covers it.
func (p Plan) ShortName() string {
	prefix := p.Recipe.Register.Prefix
	if p.CoveredBy != "" {
		prefix = p.coveredPrefix
	}
	if prefix == "" {
		return p.Name
	}
	return prefix + "-" + p.Name
}

// finish does what happens once the files are there: point the project at a
// remote, then run the recipe's commands.
//
// In that order, because a hook that pushes needs somewhere to push to.
func finish(out io.Writer, plan Plan, o CreateOptions) error {
	if err := SetUpRemote(out, plan, o); err != nil {
		return err
	}
	if o.NoHooks {
		if len(plan.Recipe.After) > 0 {
			cli.Debug("Skipping %d command(s): --no-hooks", len(plan.Recipe.After))
		}
		return nil
	}
	return RunAfter(out, plan, o.Values, o)
}
