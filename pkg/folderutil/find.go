package folderutil

import (
	"fmt"
	"io"

	"github.com/samber/lo"

	"gitlab.com/dynamo.foss/projekt/pkg/cli"
	"gitlab.com/dynamo.foss/projekt/pkg/lazypath"
)

// FindFolderByShortName finds a folder by its short name and prints its path
func FindFolderByShortName(out io.Writer, shortName string) error {
	parsedFolders, err := ParseConfig(lazypath.GetConfig())
	if err != nil {
		cli.Error("Can't parse config %v", err)
		return err
	}

	result, _ := lo.Find(parsedFolders, func(pFolder ParsedFolder) bool {
		return pFolder.ShortName == shortName
	})

	_, err = fmt.Fprintln(out, result.Path)
	return err
}
