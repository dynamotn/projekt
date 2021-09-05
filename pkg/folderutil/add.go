package folderutil

import (
	"gitlab.com/dynamo.foss/projekt/pkg/lazypath"
)

func ImportFolderToConfig(path string, prefix string, asWorkspace bool) error {
	lazypath.AddFolderConfig(path, prefix, asWorkspace)
	return nil
}
