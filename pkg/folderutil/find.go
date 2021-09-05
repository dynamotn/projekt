package folderutil

import (
	"fmt"
	"io"

	"gitlab.com/dynamo.foss/projekt/pkg/cli"
	"gitlab.com/dynamo.foss/projekt/pkg/lazypath"
)

func FindFolderByShortName(out io.Writer, shortName string) error {
	parsedFolders, err := ParseConfig(lazypath.GetConfig())
	if err != nil {
		cli.Error("Can't parse config", err)
		return err
	}

	result := ""

	//TODO: Change to binary search
	for _, pFolder := range parsedFolders {
		if pFolder.ShortName == shortName {
			result = pFolder.Path
			break
		}
	}

	fmt.Fprintln(out, result)
	return nil
}
