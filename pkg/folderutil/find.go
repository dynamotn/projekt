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

// GetOptions drives `projekt folder get`.
type GetOptions struct {
	// NoRecord leaves the jump history alone, for a lookup that is not a jump.
	NoRecord bool
}

// FindFolderByShortName finds a folder by its short name and prints its path.
//
// The name "-" means the project visited before the current one, which is what
// `pj -` asks for: the jump back.
func FindFolderByShortName(out io.Writer, shortName string, o GetOptions) error {
	if shortName == lazypath.PreviousName {
		previous, ok := lazypath.PreviousVisit()
		if !ok {
			return fmt.Errorf("no previous project to go back to")
		}
		shortName = previous.Name
	}

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

	if _, err := fmt.Fprintln(out, result.Path); err != nil {
		return err
	}

	if !o.NoRecord {
		// A history that cannot be written costs the order of a listing, not
		// the jump: report it and carry on.
		if err := lazypath.RecordVisit(result.ShortName, result.Path); err != nil {
			cli.Debug("Cannot record the jump: %v", err)
		}
	}
	return nil
}
