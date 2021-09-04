package folder

import (
	"gitlab.com/dynamo.foss/projekt/pkg/projekt"
)

func ImportFolderToConfig(path string, prefix string, asWorkspace bool) error {
	projekt.AddFolderConfig(path, prefix, asWorkspace)
	return nil
}
