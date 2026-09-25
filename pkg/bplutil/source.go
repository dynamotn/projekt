package bplutil

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	"gitlab.com/dynamo-tools/projekt/pkg/cli"
	"gitlab.com/dynamo-tools/projekt/pkg/folderutil"
	"gitlab.com/dynamo-tools/projekt/pkg/tplutil"
)

// createFromRepo clones a starting point and puts its files in the
// destination.
//
// The clone is shallow and lands in a temporary folder, from which the files
// are either copied as they are or rendered. Either way `.git` is left behind:
// the new project is not a fork of the starting point, and its first commit is
// its own.
func createFromRepo(plan Plan, o CreateOptions, log io.Writer) ([]string, error) {
	ref, err := folderutil.ParseRepoRef(plan.Recipe.Source.Repo)
	if err != nil {
		return nil, err
	}
	primary, fallback, err := ref.URLs()
	if err != nil {
		return nil, err
	}

	if o.DryRun {
		verb := "copy"
		if plan.Recipe.Source.Render {
			verb = "render"
		}
		_, err := fmt.Fprintf(log, "[DRY RUN] Would clone %s and %s it into %s\n", primary, verb, plan.Path)
		return nil, err
	}

	temp, err := os.MkdirTemp("", "projekt-boilerplate-")
	if err != nil {
		return nil, fmt.Errorf("cannot make a temporary folder: %w", err)
	}
	defer os.RemoveAll(temp)

	// git clone wants to create the folder itself.
	clone := filepath.Join(temp, "clone")
	if err := folderutil.CloneWithFallback(primary, fallback, plan.Recipe.Source.Ref, clone); err != nil {
		return nil, err
	}

	if !plan.Recipe.Source.Render {
		// Verbatim, which is what a starting point from someone else's
		// repository almost always needs: its braces are its own.
		if err := tplutil.CopyTree(clone, plan.Path); err != nil {
			return nil, err
		}
		return listFiles(plan.Path)
	}

	cli.Debug("Rendering the clone of %s", primary)
	return tplutil.Render(tplutil.RenderOptions{
		Template: tplutil.Template{Name: plan.Recipe.Name, Path: clone, Kind: tplutil.KindDir},
		Dest:     plan.Path,
		Name:     plan.Name,
		Values:   o.Values,
		Force:    o.Force,
		Out:      o.Out,
	})
}

// listFiles reports what a verbatim copy produced, so that the command can
// print it the way a render does.
func listFiles(root string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("cannot read %s: %w", root, err)
	}
	sort.Strings(files)
	return files, nil
}
