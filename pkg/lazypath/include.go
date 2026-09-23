package lazypath

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	yaml "go.yaml.in/yaml/v3"

	"gitlab.com/dynamo.foss/projekt/pkg/cli"
)

// includeDepth is how far one file may reach through another. A configuration
// is a list of folders, not a module system, and a cycle would otherwise hang.
const includeDepth = 8

// HostnameOverride, when it exists beside the configuration file, is merged
// after it without being listed: `config.<hostname>.yaml`.
//
// One dotfiles repository, two machines, and the folders that only exist on
// one of them do not have to be conditional.
func HostnameOverride(configFile string) string {
	host, err := os.Hostname()
	if err != nil || strings.TrimSpace(host) == "" {
		return ""
	}
	// Only the first element: "laptop.local" and "laptop" are the same machine
	// depending on the network it is on.
	host = strings.Split(host, ".")[0]

	dir := filepath.Dir(configFile)
	base := strings.TrimSuffix(filepath.Base(configFile), filepath.Ext(configFile))
	return filepath.Join(dir, fmt.Sprintf("%s.%s%s", base, host, filepath.Ext(configFile)))
}

// mergeIncludes returns the configuration a command reads: this machine's file
// first, then everything it includes, then the hostname override.
//
// The order is what a reader expects and what `priority` already works on: the
// file you are looking at comes first.
func mergeIncludes(own Config, configFile string) Config {
	merged := own
	seen := map[string]struct{}{cleanPath(configFile): {}}

	for _, include := range includePaths(own.Include, configFile) {
		merged = mergeOne(merged, include, seen, includeDepth)
	}
	if override := HostnameOverride(configFile); override != "" {
		if _, err := os.Stat(override); err == nil {
			merged = mergeOne(merged, override, seen, includeDepth)
		}
	}

	return merged
}

// mergeOne reads one included file and appends what it holds.
func mergeOne(into Config, path string, seen map[string]struct{}, depth int) Config {
	if depth <= 0 {
		cli.Warn("Not following %s: includes are nested too deeply", path)
		return into
	}
	key := cleanPath(path)
	if _, done := seen[key]; done {
		// Already read, whether through a cycle or through two files that
		// both include it. Reading it twice would double every folder in it.
		cli.Debug("Already included: %s", path)
		return into
	}
	seen[key] = struct{}{}

	data, err := os.ReadFile(path)
	if err != nil {
		cli.Warn("Cannot read included config %s: %v", path, err)
		return into
	}

	var included Config
	if err := yaml.Unmarshal(data, &included); err != nil {
		cli.Warn("Cannot parse included config %s: %v", path, err)
		return into
	}

	into.Folders = append(into.Folders, included.Folders...)
	into.GitServers = append(into.GitServers, included.GitServers...)
	into.Worktrees = append(into.Worktrees, included.Worktrees...)

	for _, nested := range includePaths(included.Include, path) {
		into = mergeOne(into, nested, seen, depth-1)
	}
	return into
}

// includePaths resolves what a file includes, relative to that file.
func includePaths(includes []string, configFile string) []string {
	dir := filepath.Dir(configFile)

	paths := make([]string, 0, len(includes))
	for _, include := range includes {
		include = strings.TrimSpace(include)
		if include == "" {
			continue
		}
		if strings.HasPrefix(include, "~") {
			expanded, err := NormalizePath(include)
			if err != nil {
				cli.Warn("Cannot resolve included config %s: %v", include, err)
				continue
			}
			paths = append(paths, expanded)
			continue
		}
		if !filepath.IsAbs(include) {
			// Relative to the file that names it, so a dotfiles repository can
			// be checked out anywhere.
			include = filepath.Join(dir, include)
		}
		paths = append(paths, filepath.Clean(include))
	}

	return paths
}

// IncludedFiles returns the files that are merged into this machine's
// configuration, in the order they are read. It is what `config check` reports
// and what `doctor` counts.
func IncludedFiles() []string {
	unmarshalConfig()

	configFile := ConfigFile()
	seen := map[string]struct{}{cleanPath(configFile): {}}

	var files []string
	var walk func(includes []string, from string, depth int)
	walk = func(includes []string, from string, depth int) {
		if depth <= 0 {
			return
		}
		for _, path := range includePaths(includes, from) {
			key := cleanPath(path)
			if _, done := seen[key]; done {
				continue
			}
			seen[key] = struct{}{}
			files = append(files, path)

			data, err := os.ReadFile(path)
			if err != nil {
				continue
			}
			var included Config
			if err := yaml.Unmarshal(data, &included); err != nil {
				continue
			}
			walk(included.Include, path, depth-1)
		}
	}
	walk(c.Include, configFile, includeDepth)

	if override := HostnameOverride(configFile); override != "" {
		if _, err := os.Stat(override); err == nil {
			if _, done := seen[cleanPath(override)]; !done {
				files = append(files, override)
			}
		}
	}

	return files
}

// refreshEffective recomputes what commands read after this machine's
// configuration has been written to. The includes have not changed, but the
// local part has, and the merged view is what every reader gets.
func refreshEffective() {
	effective = mergeIncludes(c, ConfigFile())
}
