package folderutil

import (
	"path/filepath"
	"strings"

	"gitlab.com/dynamo-tools/projekt/pkg/lazypath"
)

// ProjectAt returns the project a path is in, or will be: a folder that does
// not exist yet directly under a configured workspace, with a name the
// workspace's pattern accepts, is answered as it will be listed.
//
// A configuration that cannot be read answers false: guessing from half of it
// would name the wrong project.
func ProjectAt(path string) (ParsedFolder, bool) {
	if lazypath.LoadError() != nil {
		return ParsedFolder{}, false
	}
	project, err := CurrentProject(path)
	found := err == nil
	// A workspace can sit inside a configured project: the deeper answer is
	// the one the path is in.
	if child, ok := workspaceChild(path); ok && (!found || len(child.Path) > len(project.Path)) {
		return child, true
	}
	return project, found
}

// workspaceChild returns the project a path is in, or will be, directly
// under a configured workspace.
func workspaceChild(path string) (ParsedFolder, bool) {
	path, err := lazypath.NormalizePath(path)
	if err != nil {
		return ParsedFolder{}, false
	}
	path = lazypath.Canonical(path)

	for child := path; filepath.Dir(child) != child; child = filepath.Dir(child) {
		parent := filepath.Dir(child)
		for _, folder := range lazypath.GetConfig().Folders {
			if !folder.IsWorkspace || lazypath.Canonical(filepath.Clean(folder.Path)) != parent {
				continue
			}
			re, err := folder.Regex()
			if err != nil || !re.MatchString(filepath.Base(child)) {
				return ParsedFolder{}, false
			}
			workspace := filepath.Clean(folder.Path)
			return ParsedFolder{
				ShortName:   folderPrefix(folder) + filepath.Base(child),
				Path:        filepath.Join(workspace, filepath.Base(child)),
				Workspace:   workspace,
				Tags:        folder.GetTags(),
				Description: strings.TrimSpace(folder.Description),
			}, true
		}
	}
	return ParsedFolder{}, false
}
