// Package bplutil creates a project from a boilerplate recipe.
//
// A recipe is a YAML file in $XDG_DATA_HOME/projekt/boilerplates. It names
// where the files come from, what to ask for, and how the finished project is
// registered in the projekt configuration, so that `pj <name>` reaches it
// straight away.
//
// Example usage:
//
//	recipe, err := bplutil.Get("go-cli")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	result, err := bplutil.Create(bplutil.CreateOptions{
//	    Recipe: recipe,
//	    Name:   "myapp",
//	    Values: tplutil.Values{"module": "example.com/myapp"},
//	})
package bplutil

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/OpenPeeDeeP/xdg"
	yaml "go.yaml.in/yaml/v3"

	"gitlab.com/dynamo-tools/projekt/pkg/cli"
	"gitlab.com/dynamo-tools/projekt/pkg/folderutil"
	"gitlab.com/dynamo-tools/projekt/pkg/tplutil"
)

// RecipeDir is the folder recipes are read from. It is bound to the
// --boilerplate-dir flag; when empty the XDG data home is used.
var RecipeDir string

// Source says where a project's files come from. Exactly one of the fields is
// set.
type Source struct {
	// Template renders a folder template of the `t` store.
	Template string `yaml:"template"`
	// Repo clones a starting point: a URL, or `server:group/name` resolved
	// against the configured git servers.
	Repo string `yaml:"repo"`
	// Ref is the branch or tag of Repo.
	Ref string `yaml:"ref"`
	// Render runs a cloned starting point through the template engine.
	//
	// Off by default: someone else's repository is full of braces that are
	// its own, and copying it verbatim is what a starting point usually
	// means. A repository written to be a template says so here.
	Render bool `yaml:"render"`
}

// Register says how the finished project enters the configuration.
type Register struct {
	// Workspace is the folder the project is created in when only a name is
	// given. A "~" is expanded.
	Workspace string `yaml:"workspace"`
	// Prefix, Tags and Priority are the folder entry's, when one is added.
	Prefix   string   `yaml:"prefix"`
	Tags     []string `yaml:"tags"`
	Priority uint16   `yaml:"priority"`
	// Skip leaves the configuration alone, for a project that is not meant to
	// be jumped to.
	Skip bool `yaml:"skip"`
	// Remote makes the new project a git repository and points it at one.
	Remote *Remote `yaml:"remote"`
}

// Remote is where a new project will be pushed.
//
// The repository is not created on the server: that needs an API, a token and
// a network, none of which this tool has any business holding. `gh repo
// create` in an `after` hook is the way, and it is the way on purpose.
type Remote struct {
	// Host is one of the configured gitServers.
	Host string `yaml:"host"`
	// Group is the organisation or user the repository belongs to.
	Group string `yaml:"group"`
	// Name defaults to the project name.
	Name string `yaml:"name"`
	// Branch is the initial branch, when the project is not a repository yet.
	Branch string `yaml:"branch"`
	// InRepos records the new repository under the workspace's git section,
	// so `folder sync` reproduces it on the next machine.
	InRepos bool `yaml:"inRepos"`
}

// Recipe is one entry of the boilerplate store.
type Recipe struct {
	// Name is how the recipe is referred to on the command line. It is the
	// file name, so that two recipes cannot claim the same name.
	Name string `yaml:"-"`
	// Path is the absolute path of the recipe file.
	Path string `yaml:"-"`

	Description string        `yaml:"description"`
	Source      Source        `yaml:"source"`
	Vars        []tplutil.Var `yaml:"vars"`
	Register    Register      `yaml:"register"`
	// After are commands run in the new project once it exists. They are
	// rendered like anything else, so they can use the values.
	//
	// A recipe that runs commands is a recipe that runs commands: every one
	// is printed before it runs, `b show` prints them, `--dry-run` lists them
	// without running any, and `--no-hooks` skips them.
	After []string `yaml:"after"`
}

// DefaultRecipeDir returns the folder recipes live in when no override is
// given: $XDG_DATA_HOME/projekt/boilerplates.
func DefaultRecipeDir() string {
	return filepath.Join(xdg.DataHome(), "projekt", "boilerplates")
}

// Dir returns the recipe folder to use, honouring --boilerplate-dir first and
// the PROJEKT_BOILERPLATE_DIR environment variable second.
func Dir() (string, error) {
	dir := RecipeDir
	if dir == "" {
		dir = os.Getenv("PROJEKT_BOILERPLATE_DIR")
	}
	if dir == "" {
		return DefaultRecipeDir(), nil
	}
	return normalizeDir(dir)
}

// normalizeDir expands a leading "~" and makes the path absolute.
func normalizeDir(dir string) (string, error) {
	dir = strings.TrimSpace(dir)
	if dir == "" {
		return "", fmt.Errorf("boilerplate folder is empty")
	}
	if dir == "~" || strings.HasPrefix(dir, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("cannot resolve home directory: %w", err)
		}
		dir = filepath.Join(home, strings.TrimPrefix(dir, "~"))
	}
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", fmt.Errorf("cannot resolve boilerplate folder %s: %w", dir, err)
	}
	return filepath.Clean(abs), nil
}

// recipeName returns the recipe name a file is known by, or "" when the file
// is not a recipe.
func recipeName(fileName string) string {
	for _, ext := range []string{".yaml", ".yml"} {
		if strings.HasSuffix(fileName, ext) {
			return strings.TrimSuffix(fileName, ext)
		}
	}
	return ""
}

// List returns every recipe of the store, sorted by name.
//
// A missing store is not an error: it only means no recipe has been written
// yet, and every command reports that in its own way.
func List() ([]Recipe, error) {
	dir, err := Dir()
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			cli.Debug("Boilerplate folder does not exist: %s", dir)
			return nil, nil
		}
		return nil, fmt.Errorf("cannot read boilerplate folder %s: %w", dir, err)
	}

	recipes := make([]Recipe, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		name := recipeName(entry.Name())
		if name == "" {
			continue
		}

		recipe, err := load(filepath.Join(dir, entry.Name()), name)
		if err != nil {
			// One broken recipe must not hide the rest: the command that reads
			// it by name reports the parse error in full.
			cli.Warn("Skipping boilerplate %s: %v", name, err)
			continue
		}
		recipes = append(recipes, recipe)
	}

	sort.Slice(recipes, func(i, j int) bool { return recipes[i].Name < recipes[j].Name })
	return recipes, nil
}

// Get returns the recipe with the given name.
func Get(name string) (Recipe, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Recipe{}, fmt.Errorf("boilerplate name is empty")
	}
	// A name is a single store entry, never a path.
	if name != filepath.Base(name) || name == "." || name == ".." {
		return Recipe{}, fmt.Errorf("invalid boilerplate name %q", name)
	}

	dir, err := Dir()
	if err != nil {
		return Recipe{}, err
	}

	for _, ext := range []string{".yaml", ".yml"} {
		path := filepath.Join(dir, name+ext)
		if _, err := os.Stat(path); err == nil {
			return load(path, name)
		} else if !os.IsNotExist(err) {
			return Recipe{}, fmt.Errorf("cannot access %s: %w", path, err)
		}
	}
	return Recipe{}, fmt.Errorf("boilerplate %q not found in %s", name, dir)
}

// load reads and validates one recipe file.
func load(path, name string) (Recipe, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Recipe{}, fmt.Errorf("cannot read %s: %w", path, err)
	}

	var recipe Recipe
	if err := yaml.Unmarshal(data, &recipe); err != nil {
		return Recipe{}, fmt.Errorf("cannot parse %s: %w", path, err)
	}
	recipe.Name = name
	recipe.Path = path

	if err := recipe.Validate(); err != nil {
		return Recipe{}, fmt.Errorf("%s: %w", path, err)
	}
	return recipe, nil
}

// Validate reports what makes a recipe unusable.
func (r Recipe) Validate() error {
	declared := 0
	for _, set := range []bool{r.Source.Template != "", r.Source.Repo != ""} {
		if set {
			declared++
		}
	}
	switch {
	case declared == 0:
		return fmt.Errorf("source declares nothing to create from, expected source.template or source.repo")
	case declared > 1:
		return fmt.Errorf("source declares more than one origin, expected exactly one")
	}

	if r.Source.Repo != "" {
		if _, err := folderutil.ParseRepoRef(r.Source.Repo); err != nil {
			return err
		}
	}
	if r.Register.Remote != nil {
		if strings.TrimSpace(r.Register.Remote.Host) == "" {
			return fmt.Errorf("register.remote has no host")
		}
		if strings.TrimSpace(r.Register.Remote.Group) == "" {
			return fmt.Errorf("register.remote has no group")
		}
	}

	for i, v := range r.Vars {
		if strings.TrimSpace(v.Name) == "" {
			return fmt.Errorf("variable at index %d has no name", i)
		}
		if v.Type == tplutil.VarChoice && len(v.Choices) == 0 {
			return fmt.Errorf("variable %q is a choice with no choices", v.Name)
		}
	}
	return nil
}

// Origin describes where the files come from, for a listing.
func (r Recipe) Origin() string {
	switch {
	case r.Source.Template != "":
		return "template:" + r.Source.Template
	case r.Source.Repo != "":
		return "repo:" + r.Source.Repo
	default:
		return ""
	}
}
