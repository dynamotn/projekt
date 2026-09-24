package folderutil

import (
	"fmt"
	"io"
	"os"

	"gitlab.com/dynamo.foss/projekt/pkg/lazypath"
)

// MoveOptions drives `projekt folder move`.
type MoveOptions struct {
	// DryRun says what would happen without moving or writing anything.
	DryRun bool
}

// MoveFolder points a project at a new path, moving the files when they are
// still at the old one.
//
// Both halves of the job are the same command because both happen: sometimes
// the folder has already been moved by hand and only the configuration is
// behind, and sometimes moving it is the thing being asked for.
func MoveFolder(out io.Writer, shortName, to string, o MoveOptions) error {
	if err := lazypath.LoadError(); err != nil {
		return err
	}

	folder, err := FindFolder(shortName)
	if err != nil {
		return err
	}
	if _, _, isWorktree := lazypath.SplitWorktreeName(shortName); isWorktree {
		return fmt.Errorf("%s is a worktree; remove it and add it where you want it", shortName)
	}
	if _, _, own := lazypath.FindOwnFolder(folder.Path); !own {
		return fmt.Errorf("%s has no entry of its own: it is found inside a workspace, so move the folder and it follows", shortName)
	}

	target, err := lazypath.NormalizePath(to)
	if err != nil {
		return err
	}
	if target == folder.Path {
		return fmt.Errorf("%s is already at %s", shortName, target)
	}

	_, sourceErr := os.Stat(folder.Path)
	_, targetErr := os.Stat(target)
	sourceThere := sourceErr == nil
	targetThere := targetErr == nil

	switch {
	case targetThere && sourceThere:
		return fmt.Errorf("both %s and %s are there; remove one, or say which with `folder remove` and `folder add`",
			folder.Path, target)
	case !targetThere && !sourceThere:
		return fmt.Errorf("neither %s nor %s is there", folder.Path, target)
	}

	if o.DryRun {
		if sourceThere {
			_, err := fmt.Fprintf(out, "[DRY RUN] Would move %s to %s and follow it in the configuration\n",
				folder.Path, target)
			return err
		}
		_, err := fmt.Fprintf(out, "[DRY RUN] Would point %s at %s, which is where it already is\n",
			shortName, target)
		return err
	}

	if sourceThere {
		if err := moveFolder(folder.Path, target); err != nil {
			return err
		}
	}

	// The configuration follows only once the files are where it will say they
	// are, so a move that failed leaves a configuration that is still true.
	if err := lazypath.MoveFolder(folder.Path, target); err != nil {
		return err
	}

	_, err = fmt.Fprintf(out, "%s is now at %s\n", shortName, target)
	return err
}
