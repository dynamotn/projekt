package folderutil

import (
	"fmt"
	"io"

	"github.com/samber/lo"

	"gitlab.com/dynamo.foss/projekt/pkg/cli"
	"gitlab.com/dynamo.foss/projekt/pkg/lazypath"
)

// ErrFolderNotFound is returned when no configured folder matches a short name.
var ErrFolderNotFound = fmt.Errorf("no project folder found")

// FindFolderByShortName finds a folder by its short name and prints its path
func FindFolderByShortName(out io.Writer, shortName string) error {
	parsedFolders, err := ParseConfig(lazypath.GetConfig())
	if err != nil {
		cli.Error("Can't parse config: %v", err)
		return err
	}

	result, ok := lo.Find(parsedFolders, func(pFolder ParsedFolder) bool {
		return pFolder.ShortName == shortName
	})
	if !ok {
		// Report the miss instead of printing an empty path: callers such as the
		// `pj` shell function rely on the exit code to detect an unknown project.
		return fmt.Errorf("%w with short name %q", ErrFolderNotFound, shortName)
	}

	_, err = fmt.Fprintln(out, result.Path)
	return err
}
