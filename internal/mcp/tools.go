package mcp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"regexp"
	"strings"

	_ "embed"

	"gitlab.com/dynamo-tools/projekt/internal/version"
	"gitlab.com/dynamo-tools/projekt/pkg/cli"
	"gitlab.com/dynamo-tools/projekt/pkg/folderutil"
	"gitlab.com/dynamo-tools/projekt/pkg/lazypath"
	"gitlab.com/dynamo-tools/projekt/pkg/showutil"
)

//go:embed instructions.txt
var instructions string

// NewServer builds the projekt server; write adds the write tools.
func NewServer(write bool) *Server {
	tools := ReadTools()
	if write {
		tools = append(tools, WriteTools()...)
	}
	return &Server{
		Name: "projekt", Version: version.GetVersionStr(), Instructions: instructions,
		Tools: tools, Before: lazypath.ReloadConfig,
	}
}

// ReadTools are the tools that change nothing.
func ReadTools() []Tool {
	tags := prop("array", "Keep only the projects carrying every one of these tags")
	tags["items"] = map[string]any{"type": "string"}
	return []Tool{
		{
			Name:        "project_list",
			Description: "List the projects: short name, path, workspace, tags and description.",
			Schema: schema(nil, map[string]any{
				"tags":  tags,
				"query": prop("string", "Keep only the projects whose short name contains this, ignoring case"),
			}),
			ReadOnly: true, Call: projectList,
		},
		{
			Name:        "project_show",
			Description: "Everything projekt knows about one project: path, tags, description, note, working trees and git state.",
			Schema: schema([]string{"name"}, map[string]any{
				"name": prop("string", "The short name of the project"),
			}),
			ReadOnly: true, Call: projectShow,
		},
		{
			Name:        "project_status",
			Description: "The git state of the projects: branch, changes, commits not pushed, stashes. Reads many repositories; narrow it with tags or since.",
			Schema: schema(nil, map[string]any{
				"tags":     tags,
				"dirty":    prop("boolean", "Keep only the projects with something to attend to"),
				"since":    prop("string", "Keep only the projects with a commit or a jump this recent, such as 12h, 7d or 2w"),
				"branches": prop("boolean", "Add how the branch stands against the default one"),
			}),
			ReadOnly: true, Call: projectStatus,
		},
		{
			Name:        "project_recent",
			Description: "The projects the owner jumped to, most recent first.",
			Schema:      schema(nil, map[string]any{"limit": prop("integer", "How many to return; all when omitted")}),
			ReadOnly:    true, Call: projectRecent,
		},
		{
			Name:        "project_current",
			Description: "The project a path is inside; an error when it is in none.",
			Schema: schema([]string{"path"}, map[string]any{
				"path": prop("string", "An absolute path"),
			}),
			ReadOnly: true, Call: projectCurrent,
		},
	}
}

// WriteTools are the tools that change something; NewServer offers them only
// when asked.
func WriteTools() []Tool {
	return []Tool{
		{
			Name:        "note_add",
			Description: "Append a dated line to a project's note. Newlines in the text become spaces: one call adds one line.",
			Schema: schema([]string{"name", "text"}, map[string]any{
				"name": prop("string", "The short name of the project"),
				"text": prop("string", "What to note"),
			}),
			Call: noteAdd,
		},
		{
			Name:        "describe_set",
			Description: "Set the one-line description of a project in the configuration.",
			Schema: schema([]string{"name", "description"}, map[string]any{
				"name":        prop("string", "The short name of the project"),
				"description": prop("string", "The new description, one line"),
			}),
			Call: describeSet,
		},
		{
			Name:        "worktree_add",
			Description: "Create a git working tree of a project on a branch, creating the branch from HEAD when it does not exist, and record it.",
			Schema: schema([]string{"project", "branch"}, map[string]any{
				"project": prop("string", "The short name of the project"),
				"branch":  prop("string", "The branch to check out"),
				"name":    prop("string", "The name after the @; the last element of the branch when omitted"),
			}),
			Call: worktreeAdd,
		},
	}
}

func prop(typ, description string) map[string]any {
	return map[string]any{"type": typ, "description": description}
}

func schema(required []string, properties map[string]any) map[string]any {
	s := map[string]any{"type": "object", "properties": properties, "additionalProperties": false}
	if len(required) > 0 {
		s["required"] = required
	}
	return s
}

// decodeArgs reads arguments into v, refusing an unknown one.
func decodeArgs(arguments json.RawMessage, v any) error {
	if len(bytes.TrimSpace(arguments)) == 0 {
		arguments = json.RawMessage("{}")
	}
	dec := json.NewDecoder(bytes.NewReader(arguments))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return fmt.Errorf("invalid arguments: %w", err)
	}
	return nil
}

func required(name, value string) error {
	if strings.TrimSpace(value) == "" {
		return fmt.Errorf("%s is required", name)
	}
	return nil
}

func projectList(arguments json.RawMessage) (string, error) {
	var a struct {
		Tags  []string `json:"tags"`
		Query string   `json:"query"`
	}
	if err := decodeArgs(arguments, &a); err != nil {
		return "", err
	}
	var out bytes.Buffer
	if err := folderutil.ListFolders(&out, &folderutil.ListOption{Output: cli.OutputJSON, Tags: a.Tags}); err != nil {
		return "", err
	}
	if a.Query == "" {
		return out.String(), nil
	}
	var all []map[string]any
	if err := json.Unmarshal(out.Bytes(), &all); err != nil {
		return "", err
	}
	kept := []map[string]any{}
	for _, p := range all {
		name, _ := p["shortName"].(string)
		if strings.Contains(strings.ToLower(name), strings.ToLower(a.Query)) {
			kept = append(kept, p)
		}
	}
	return encode(kept)
}

// encode writes JSON the way the CLI's listings do.
func encode(v any) (string, error) {
	var out bytes.Buffer
	enc := json.NewEncoder(&out)
	enc.SetIndent("", "  ")
	err := enc.Encode(v)
	return out.String(), err
}

func projectShow(arguments json.RawMessage) (string, error) {
	var a struct {
		Name string `json:"name"`
	}
	if err := decodeArgs(arguments, &a); err != nil {
		return "", err
	}
	if err := required("name", a.Name); err != nil {
		return "", err
	}
	var out bytes.Buffer
	err := showutil.Show(&out, a.Name, showutil.Options{ListOutputOption: cli.ListOutputOption{Output: cli.OutputJSON}})
	return out.String(), err
}

func projectStatus(arguments json.RawMessage) (string, error) {
	var a struct {
		Tags     []string `json:"tags"`
		Dirty    bool     `json:"dirty"`
		Since    string   `json:"since"`
		Branches bool     `json:"branches"`
	}
	if err := decodeArgs(arguments, &a); err != nil {
		return "", err
	}
	since, err := folderutil.ParseSince(a.Since)
	if err != nil {
		return "", err
	}
	var out bytes.Buffer
	// Requests stays off: it is a network call per project.
	err = folderutil.StatusFolders(&out, &folderutil.StatusOptions{
		Tags: a.Tags, DirtyOnly: a.Dirty, Since: since, Branches: a.Branches,
		ListOutputOption: cli.ListOutputOption{Output: cli.OutputJSON},
	})
	return out.String(), err
}

func projectRecent(arguments json.RawMessage) (string, error) {
	var a struct {
		Limit int `json:"limit"`
	}
	if err := decodeArgs(arguments, &a); err != nil {
		return "", err
	}
	var out bytes.Buffer
	err := folderutil.ListRecent(&out, &folderutil.RecentOptions{
		Limit: a.Limit, ListOutputOption: cli.ListOutputOption{Output: cli.OutputJSON},
	})
	return out.String(), err
}

func projectCurrent(arguments json.RawMessage) (string, error) {
	var a struct {
		Path string `json:"path"`
	}
	if err := decodeArgs(arguments, &a); err != nil {
		return "", err
	}
	if err := required("path", a.Path); err != nil {
		return "", err
	}
	p, err := folderutil.CurrentProject(a.Path)
	if err != nil {
		return "", err
	}
	tags := p.Tags
	if tags == nil {
		tags = []string{}
	}
	return encode(map[string]any{
		"shortName": p.ShortName, "path": p.Path, "workspace": p.Workspace,
		"tags": tags, "description": p.Description,
	})
}

// lineBreaks is a run of CR and LF: text with one in it could forge a dated
// line of its own in the note.
var lineBreaks = regexp.MustCompile(`[\r\n]+`)

// The write tools check name and project themselves: an empty short name
// means the folder the server runs in, which an agent must never reach.

func noteAdd(arguments json.RawMessage) (string, error) {
	var a struct {
		Name string `json:"name"`
		Text string `json:"text"`
	}
	if err := decodeArgs(arguments, &a); err != nil {
		return "", err
	}
	if err := required("name", a.Name); err != nil {
		return "", err
	}
	text := strings.TrimSpace(lineBreaks.ReplaceAllString(a.Text, " "))
	if err := required("text", text); err != nil {
		return "", err
	}
	if err := folderutil.NoteFolder(io.Discard, a.Name, folderutil.NoteOptions{Add: text}); err != nil {
		return "", err
	}
	return encode(map[string]any{"noted": a.Name})
}

func describeSet(arguments json.RawMessage) (string, error) {
	var a struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	if err := decodeArgs(arguments, &a); err != nil {
		return "", err
	}
	if err := required("name", a.Name); err != nil {
		return "", err
	}
	// Without text DescribeFolder would only print the description.
	if err := required("description", a.Description); err != nil {
		return "", err
	}
	if err := folderutil.DescribeFolder(io.Discard, a.Name, a.Description, false); err != nil {
		return "", err
	}
	return encode(map[string]any{"described": a.Name})
}

func worktreeAdd(arguments json.RawMessage) (string, error) {
	var a struct {
		Project string `json:"project"`
		Branch  string `json:"branch"`
		Name    string `json:"name"`
	}
	if err := decodeArgs(arguments, &a); err != nil {
		return "", err
	}
	if err := required("project", a.Project); err != nil {
		return "", err
	}
	if err := required("branch", a.Branch); err != nil {
		return "", err
	}
	w, err := folderutil.AddWorktree(io.Discard, folderutil.AddWorktreeOptions{Project: a.Project, Branch: a.Branch, Name: a.Name})
	if err != nil {
		return "", err
	}
	return encode(map[string]any{"name": w.ShortName(), "path": w.Path})
}
