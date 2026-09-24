package folderutil

import (
	"fmt"
	"io"

	"gitlab.com/dynamo.foss/projekt/pkg/cli"
	"gitlab.com/dynamo.foss/projekt/pkg/lazypath"
)

// ErrFolderNotFound is returned when no configured folder matches a short name.
var ErrFolderNotFound = fmt.Errorf("no project folder found")

// GetOptions drives `projekt folder get`.
type GetOptions struct {
	// NoRecord leaves the jump history alone, for a lookup that is not a jump.
	NoRecord bool
	// ExactOnly turns off the looser matching, for a script that would rather
	// be told it was wrong than sent somewhere close.
	ExactOnly bool
}

// FindFolderByShortName finds a folder by its short name and prints its path.
//
// The name "-" means the project visited before the current one, which is what
// `pj -` asks for: the jump back.
//
// A name that is not one of a project exactly is matched loosely — by prefix,
// then anywhere in the name, then letter by letter — and the best candidate
// wins. An exact name is never reinterpreted, so nothing you type in full can
// take you somewhere else.
func FindFolderByShortName(out io.Writer, shortName string, o GetOptions) error {
	if shortName == lazypath.PreviousName {
		previous, ok := lazypath.PreviousVisit()
		if !ok {
			return fmt.Errorf("no previous project to go back to")
		}
		shortName = previous.Name
	}

	folder, err := ResolveFolder(shortName, o)
	if err != nil {
		return err
	}

	if _, err := fmt.Fprintln(out, folder.Path); err != nil {
		return err
	}

	if !o.NoRecord {
		// A history that cannot be written costs the order of a listing, not
		// the jump: report it and carry on.
		if err := lazypath.RecordVisit(folder.ShortName, folder.Path); err != nil {
			cli.Debug("Cannot record the jump: %v", err)
		}
	}
	return nil
}

// ResolveFolder returns the project a name means.
func ResolveFolder(shortName string, o GetOptions) (ParsedFolder, error) {
	parsedFolders, err := ParseConfig(lazypath.GetConfig())
	if err != nil {
		cli.Error("Can't parse config: %v", err)
		return ParsedFolder{}, err
	}

	matches := MatchFolders(parsedFolders, shortName)
	if len(matches) == 0 {
		// Report the miss instead of printing an empty path: callers such as the
		// `pj` shell function rely on the exit code to detect an unknown project.
		return ParsedFolder{}, fmt.Errorf("%w with short name %q", ErrFolderNotFound, shortName)
	}

	best := matches[0]
	if o.ExactOnly && !best.Exact() {
		return ParsedFolder{}, fmt.Errorf("%w with short name %q", ErrFolderNotFound, shortName)
	}

	if !best.Exact() && len(matches) > 1 {
		// Say what else it could have been, so that a wrong guess is one
		// `--verbose debug` away from being explained rather than a mystery.
		others := make([]string, 0, len(matches)-1)
		for _, match := range matches[1:] {
			others = append(others, match.Folder.ShortName)
		}
		cli.Debug("%q also matched: %v", shortName, others)
	}

	return best.Folder, nil
}
