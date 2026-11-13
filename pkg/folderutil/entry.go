package folderutil

import (
	"path/filepath"
	"strings"

	"gitlab.com/dynamo-tools/projekt/pkg/lazypath"
)

// EntryFor finds the configuration entry a project's hooks come from: the
// deepest entry whose path holds the project, which is the project's own
// entry, or the workspace a child or a working tree hangs under.
func EntryFor(entries []lazypath.Folder, folder ParsedFolder) (lazypath.Folder, bool) {
	var best lazypath.Folder
	var found bool
	for _, candidate := range []string{folder.Path, folder.Workspace} {
		target := lazypath.Canonical(candidate)
		for _, entry := range entries {
			path := lazypath.Canonical(entry.Path)
			if !holds(path, target) {
				continue
			}
			if !found || len(path) > len(lazypath.Canonical(best.Path)) {
				best, found = entry, true
			}
		}
		if found {
			return best, true
		}
	}
	return lazypath.Folder{}, false
}

func holds(folder, path string) bool {
	folder, path = filepath.Clean(folder), filepath.Clean(path)
	return folder == path || strings.HasPrefix(path, folder+string(filepath.Separator))
}
