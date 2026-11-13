// Package showutil answers `projekt folder show`: everything projekt knows
// about one project, in one place.
//
// It sits above folderutil, hookutil and tplutil, which each know a part and
// which hookutil and tplutil already import folderutil for.
package showutil

import (
	"encoding/json"
	"io"
	"os"
	"sort"
	"strings"

	"gitlab.com/dynamo-tools/projekt/pkg/cli"
	"gitlab.com/dynamo-tools/projekt/pkg/folderutil"
	"gitlab.com/dynamo-tools/projekt/pkg/hookutil"
	"gitlab.com/dynamo-tools/projekt/pkg/lazypath"
	"gitlab.com/dynamo-tools/projekt/pkg/tplutil"
)

// Options drives `projekt folder show`.
type Options struct {
	// NoStatus skips the git state.
	NoStatus bool
	cli.ListOutputOption
}

// Info is what projekt knows about one project. Its JSON is also what the
// project_show tool of the MCP server returns, so the keys are a contract.
type Info struct {
	Name        string     `json:"name"`
	Path        string     `json:"path"`
	Workspace   string     `json:"workspace,omitempty"`
	Tags        []string   `json:"tags,omitempty"`
	Description string     `json:"description,omitempty"`
	Source      string     `json:"source,omitempty"`
	Note        string     `json:"note,omitempty"`
	Worktrees   []Worktree `json:"worktrees,omitempty"`
	Git         *Git       `json:"git,omitempty"`
	Hooks       *Hooks     `json:"hooks,omitempty"`
	Templates   []string   `json:"templates,omitempty"`
}

// Worktree is one working tree of the project.
type Worktree struct {
	Name   string `json:"name"`
	Branch string `json:"branch"`
	Path   string `json:"path"`
}

// Git is the state `folder status` reports, under the same JSON keys.
type Git struct {
	State   string `json:"state"`
	Branch  string `json:"branch,omitempty"`
	Changed int    `json:"changed"`
	Ahead   int    `json:"ahead"`
	Behind  int    `json:"behind"`
	Stashes int    `json:"stashes"`
	Last    string `json:"last,omitempty"`
}

// Hooks are the lines a jump into or out of the project runs.
type Hooks struct {
	Enter []string `json:"enter,omitempty"`
	Leave []string `json:"leave,omitempty"`
}

// Gather collects what projekt knows about one project. An empty name means
// the project the current folder is in.
func Gather(name string, o Options) (Info, error) {
	var folder folderutil.ParsedFolder
	var err error
	if name == "" {
		var cwd string
		if cwd, err = os.Getwd(); err != nil {
			return Info{}, err
		}
		folder, err = folderutil.CurrentProject(cwd)
	} else {
		folder, err = folderutil.FindFolder(name)
	}
	if err != nil {
		return Info{}, err
	}

	config := lazypath.GetConfig()
	info := Info{
		Name:        folder.ShortName,
		Path:        folder.Path,
		Workspace:   folder.Workspace,
		Tags:        folder.Tags,
		Description: folder.Description,
		Source:      lazypath.ConfigFile(),
	}
	if folder.Workspace == folder.Path {
		info.Workspace = ""
	}
	if entry, ok := folderutil.EntryFor(config.Folders, folder); ok && entry.Source != "" {
		info.Source = entry.Source
	}

	if info.Note, err = folderutil.ReadNote(folder.Path); err != nil {
		return Info{}, err
	}
	for _, wt := range config.Worktrees {
		if wt.Project == folder.ShortName {
			info.Worktrees = append(info.Worktrees, Worktree{Name: wt.Name, Branch: wt.Branch, Path: wt.Path})
		}
	}
	if !o.NoStatus {
		s := folderutil.StatusOf(folder)
		info.Git = &Git{State: s.State, Branch: s.Branch, Changed: s.Changed,
			Ahead: s.Ahead, Behind: s.Behind, Stashes: s.Stashes, Last: s.Last}
	}
	if set := hookutil.ForProject(folder); !set.IsEmpty() {
		info.Hooks = &Hooks{Enter: commands(set.Enter), Leave: commands(set.Leave)}
	}

	record, err := tplutil.LoadRecord(folder.Path)
	if err != nil {
		return Info{}, err
	}
	info.Templates = templatesOf(record)
	return info, nil
}

func commands(hooks []hookutil.Hook) []string {
	var lines []string
	for _, hook := range hooks {
		lines = append(lines, hook.Command)
	}
	return lines
}

// templatesOf lists what the project was rendered from, sorted, once each.
func templatesOf(record tplutil.Record) []string {
	seen := map[string]bool{}
	var names []string
	add := func(name string) {
		if name != "" && !seen[name] {
			seen[name] = true
			names = append(names, name)
		}
	}
	for _, render := range record.Renders {
		add(render.Template)
	}
	for _, recipe := range record.Recipes {
		add(recipe.Recipe)
	}
	sort.Strings(names)
	return names
}

// Show prints it: one JSON object for json; a FIELD/VALUE listing otherwise.
func Show(out io.Writer, name string, o Options) error {
	info, err := Gather(name, o)
	if err != nil {
		return err
	}
	if o.Output == cli.OutputJSON {
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(info)
	}

	view := cli.ListView{Ordered: true, Columns: []cli.ListColumn{
		{Header: "FIELD", Key: "field"},
		{Header: "VALUE", Key: "value"},
	}}
	add := func(field, value string) {
		if value != "" {
			view.AppendRow(field, value)
		}
	}
	add("name", info.Name)
	add("path", info.Path)
	add("workspace", info.Workspace)
	add("tags", strings.Join(info.Tags, ", "))
	add("description", info.Description)
	add("source", info.Source)
	add("note", headline(info.Note))
	var trees []string
	for _, wt := range info.Worktrees {
		trees = append(trees, wt.Name+" ("+wt.Branch+")")
	}
	add("worktrees", strings.Join(trees, ", "))
	if g := info.Git; g != nil {
		summary := folderutil.FolderStatus{State: g.State, Changed: g.Changed,
			Ahead: g.Ahead, Behind: g.Behind, Stashes: g.Stashes}.Summary()
		if g.Branch != "" {
			summary = g.Branch + ": " + summary
		}
		add("git", summary)
	}
	if h := info.Hooks; h != nil {
		add("hooks enter", strings.Join(h.Enter, "; "))
		add("hooks leave", strings.Join(h.Leave, "; "))
	}
	add("templates", strings.Join(info.Templates, ", "))

	return cli.EncodeList(out, view, o.ListOutputOption)
}

// headline is the first line of the note, marked when there is more.
func headline(note string) string {
	first := folderutil.NoteHeadlineOf(note)
	if first == "" {
		return ""
	}
	if strings.Count(strings.TrimSpace(note), "\n") > 0 {
		first += "…"
	}
	return first
}
