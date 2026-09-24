package folderutil

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"gitlab.com/dynamo.foss/projekt/pkg/lazypath"
)

// TagFolder adds or removes tags on a project, and prints what it now carries.
//
// A project found inside a workspace has no entry of its own and inherits the
// workspace's tags, so it is the workspace that gets tagged: saying that is
// more use than silently doing nothing.
func TagFolder(out io.Writer, shortName string, add, remove []string) error {
	if err := lazypath.LoadError(); err != nil {
		return err
	}

	folder, err := FindFolder(shortName)
	if err != nil {
		return err
	}
	if _, _, isWorktree := lazypath.SplitWorktreeName(shortName); isWorktree {
		return fmt.Errorf("%s is a worktree; it carries the tags of the project it belongs to", shortName)
	}

	own, _, isOwn := lazypath.FindOwnFolder(folder.Path)
	if !isOwn {
		return fmt.Errorf("%s has no entry of its own: it is found inside a workspace, and carries the workspace's tags", shortName)
	}

	tags := tagsAfter(own.GetTags(), add, remove)
	if err := lazypath.SetFolderTags(folder.Path, tags); err != nil {
		return err
	}

	if len(tags) == 0 {
		_, err := fmt.Fprintf(out, "%s carries no tags\n", shortName)
		return err
	}
	_, err = fmt.Fprintf(out, "%s: %s\n", shortName, strings.Join(tags, ", "))
	return err
}

// tagsAfter applies the additions and the removals, and sorts the result so
// that the configuration file does not churn on every edit.
func tagsAfter(current, add, remove []string) []string {
	set := make(map[string]struct{}, len(current)+len(add))
	for _, tag := range lazypath.NormalizeTags(current) {
		set[tag] = struct{}{}
	}
	for _, tag := range lazypath.NormalizeTags(add) {
		set[tag] = struct{}{}
	}
	for _, tag := range lazypath.NormalizeTags(remove) {
		delete(set, tag)
	}

	tags := make([]string, 0, len(set))
	for tag := range set {
		tags = append(tags, tag)
	}
	sort.Strings(tags)
	return tags
}

// ListTags prints every tag in use, and how many projects carry it.
func ListTags(out io.Writer) error {
	folders, err := ParseConfig(lazypath.GetConfig())
	if err != nil {
		return err
	}

	counts := map[string]int{}
	for _, folder := range folders {
		for _, tag := range folder.Tags {
			counts[tag]++
		}
	}
	if len(counts) == 0 {
		return nil
	}

	tags := make([]string, 0, len(counts))
	for tag := range counts {
		tags = append(tags, tag)
	}
	sort.Strings(tags)

	for _, tag := range tags {
		if _, err := fmt.Fprintf(out, "%s\t%d\n", tag, counts[tag]); err != nil {
			return err
		}
	}
	return nil
}
