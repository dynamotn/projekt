package bplutil

import (
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gitlab.com/dynamo.foss/projekt/pkg/cli"
	"gitlab.com/dynamo.foss/projekt/pkg/folderutil"
	"gitlab.com/dynamo.foss/projekt/pkg/tplutil"
)

// RepoRef is a repository to start from, however it was written.
type RepoRef struct {
	// URL is set when the recipe gave one outright.
	URL string
	// Host, Group and Name are set when it gave the shorthand
	// `server:group/name`, resolved against the configured git servers.
	Host, Group, Name string
}

// ParseRepoRef reads what a recipe means by `source.repo`.
//
// A URL is taken as it is. Anything else is `server:group/name`, where the
// server is one of the configured gitServers, so that a recipe can be shared
// between people whose remotes differ in scheme or host.
func ParseRepoRef(repo string) (RepoRef, error) {
	repo = strings.TrimSpace(repo)
	if repo == "" {
		return RepoRef{}, fmt.Errorf("source.repo is empty")
	}
	if folderutil.IsGitURL(repo) {
		return RepoRef{URL: repo}, nil
	}

	host, rest, ok := strings.Cut(repo, ":")
	if !ok || strings.TrimSpace(host) == "" {
		return RepoRef{}, fmt.Errorf("source.repo %q is neither a URL nor server:group/name", repo)
	}
	group, name, ok := strings.Cut(strings.Trim(rest, "/"), "/")
	if !ok || strings.TrimSpace(group) == "" || strings.TrimSpace(name) == "" {
		return RepoRef{}, fmt.Errorf("source.repo %q is missing the group or the repository", repo)
	}

	return RepoRef{Host: host, Group: group, Name: strings.TrimSuffix(name, ".git")}, nil
}

// String describes the reference, for a listing.
func (r RepoRef) String() string {
	if r.URL != "" {
		return r.URL
	}
	return fmt.Sprintf("%s:%s/%s", r.Host, r.Group, r.Name)
}

// URLs returns the URLs to clone from, the preferred one first.
func (r RepoRef) URLs() (primary, fallback string, err error) {
	if r.URL != "" {
		return r.URL, "", nil
	}
	return folderutil.GitURLs(r.Host, r.Group, r.Name)
}

// createFromRepo clones a starting point and puts its files in the
// destination.
//
// The clone is shallow and lands in a temporary folder, from which the files
// are either copied as they are or rendered. Either way `.git` is left behind:
// the new project is not a fork of the starting point, and its first commit is
// its own.
func createFromRepo(plan Plan, o CreateOptions, log io.Writer) ([]string, error) {
	ref, err := ParseRepoRef(plan.Recipe.Source.Repo)
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
