package lazypath

import (
	"path/filepath"
	"sort"
	"strings"

	"github.com/samber/lo"

	"gitlab.com/dynamo-tools/projekt/pkg/cli"
)

const defaultRegexWorkspace = `^[^.].+`

// GitWorktree is an extra working tree checked out from a repository, so that
// several branches of it can be open at once.
type GitWorktree struct {
	// Path is where the working tree lives, relative to the folder the
	// repository belongs to, exactly like GitRepo.Path.
	Path string `yaml:"path" mapstructure:"path"`
	// Branch is the branch to check out. It is required: without it git would
	// invent a branch name from the path, which is rarely what was meant.
	Branch string `yaml:"branch" mapstructure:"branch"`
}

type GitRepo struct {
	Name string `yaml:"name" mapstructure:"name"`
	Path string `yaml:"path" mapstructure:"path"`
	// Remotes are extra remotes to keep configured on the repository, by name.
	// The clone already sets up origin; listing origin here overrides it.
	Remotes map[string]string `yaml:"remotes,omitempty" mapstructure:"remotes"`
	// Worktrees are extra working trees to create for this repository.
	Worktrees []GitWorktree `yaml:"worktrees,omitempty" mapstructure:"worktrees"`
}

// RemoteNames returns the configured remote names in a fixed order, so that
// syncing and checking report them the same way on every run.
func (r *GitRepo) RemoteNames() []string {
	if len(r.Remotes) == 0 {
		return nil
	}

	names := make([]string, 0, len(r.Remotes))
	for name := range r.Remotes {
		names = append(names, name)
	}
	sort.Strings(names)

	return names
}

type GitConfig struct {
	Host  string    `yaml:"host" mapstructure:"host"`
	Group string    `yaml:"group" mapstructure:"group"`
	Repos []GitRepo `yaml:"repos" mapstructure:"repos"`
}

type Folder struct {
	Path        string     `yaml:"path" mapstructure:"path"`
	Name        string     `yaml:"name,omitempty" mapstructure:"name"`
	Prefix      string     `yaml:"prefix" mapstructure:"prefix"`
	IsWorkspace bool       `yaml:"is_workspace" mapstructure:"is_workspace"`
	RegexMatch  string     `yaml:"regex" mapstructure:"regex"`
	Priority    uint16     `yaml:"priority" mapstructure:"priority"`
	Tags        []string   `yaml:"tags,omitempty" mapstructure:"tags"`
	Git         *GitConfig `yaml:"git,omitempty" mapstructure:"git,omitempty"`
}

// GetTags returns the folder's tags, cleaned up. A workspace passes its tags on
// to every folder inside it, since those folders are not configured themselves.
func (f *Folder) GetTags() []string {
	return NormalizeTags(f.Tags)
}

// NormalizeTags trims each tag, drops the empty and duplicate ones, and keeps
// the order the remaining tags were given in.
//
// It is applied on both sides of a comparison, so that a tag written with stray
// whitespace in the config still matches the same tag typed on the command line.
func NormalizeTags(tags []string) []string {
	if len(tags) == 0 {
		return nil
	}

	seen := make(map[string]struct{}, len(tags))
	result := make([]string, 0, len(tags))
	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		if tag == "" {
			continue
		}
		if _, dup := seen[tag]; dup {
			continue
		}
		seen[tag] = struct{}{}
		result = append(result, tag)
	}

	if len(result) == 0 {
		return nil
	}
	return result
}

// HasTags reports whether tags contains every one of the wanted tags, which is
// the "narrow the selection" behaviour a filter needs: asking for more tags can
// only ever match fewer folders.
//
// No wanted tags means no filtering, so everything matches.
func HasTags(tags []string, want []string) bool {
	want = NormalizeTags(want)
	if len(want) == 0 {
		return true
	}

	have := make(map[string]struct{}, len(tags))
	for _, tag := range NormalizeTags(tags) {
		have[tag] = struct{}{}
	}

	for _, tag := range want {
		if _, ok := have[tag]; !ok {
			return false
		}
	}
	return true
}

// ShortName returns the name the folder itself is reachable by, without the
// prefix. It is the configured name, or the last element of the path.
//
// A workspace has no short name of its own; its children are named after their
// own directory.
func (f *Folder) ShortName() string {
	if f.Name != "" {
		return f.Name
	}
	// Clean first: the base of a path such as "/home/me/dotfiles/home/.."
	// is "..", which is not a usable name.
	return filepath.Base(cleanPath(f.Path))
}

func (f *Folder) GetRegexMatch() string {
	if !f.IsWorkspace {
		return ""
	}
	if f.RegexMatch != "" {
		return f.RegexMatch
	}
	return defaultRegexWorkspace
}

// cleanPath makes two spellings of the same path comparable.
func cleanPath(path string) string {
	trimmed := strings.TrimRight(path, "/")
	if trimmed == "" {
		return path
	}
	return filepath.Clean(trimmed)
}

// CheckFolderExist checks if a folder path exists in the configuration
func CheckFolderExist(path string) (bool, int) {
	unmarshalConfig()

	normalizedPath := cleanPath(path)
	_, index, ok := lo.FindIndexOf(c.Folders, func(folder Folder) bool {
		return cleanPath(folder.Path) == normalizedPath
	})

	return ok, index
}

// AddToConfig adds the folder to the configuration file
func (f *Folder) AddToConfig() error {
	if loadErr != nil {
		return loadErr
	}
	unmarshalConfig()
	isExisted, _ := CheckFolderExist(f.Path)

	if isExisted {
		cli.Info("%s is already existed!", f.Path)
		return nil
	}

	c.Folders = append(c.Folders, *f)
	err := saveConfig("folders")
	if err != nil {
		cli.Error("Failed to write config %v", err)
		return err
	}
	refreshEffective()

	cli.Info("Added %s to config", f.Path)
	return nil
}

// RemoveFromConfig removes a folder from the configuration by path
func RemoveFromConfig(path string) error {
	if loadErr != nil {
		return loadErr
	}
	unmarshalConfig()

	isExisted, index := CheckFolderExist(path)

	if !isExisted {
		cli.Info("%s wasn't added as project!", path)
		return nil
	}

	c.Folders = append(c.Folders[:index], c.Folders[index+1:]...)
	err := saveConfig("folders")
	if err != nil {
		cli.Error("Failed to write config %v", err)
		return err
	}
	refreshEffective()

	cli.Info("Removed %s from config", path)
	return nil
}
