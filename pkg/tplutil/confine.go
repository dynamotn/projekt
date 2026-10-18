package tplutil

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gitlab.com/dynamo-tools/projekt/pkg/lazypath"
)

// A template store may be a clone of somebody else's repository, and a
// repository can hold a symbolic link to anywhere. Reading one would copy a
// file of this machine into the output; writing through one would put the
// output somewhere else. Neither is gated by the trust a command needs, so
// both are refused here instead.

// within reports whether path is root or lives under it. Both must already be
// canonical.
func within(root, path string) bool {
	return path == root || strings.HasPrefix(path, root+string(filepath.Separator))
}

// storeRoot is the folder a template's files must stay inside: the store for
// a template of the store, the template itself for one rendered from
// anywhere else, such as a repository cloned for a boilerplate.
func storeRoot(tpl Template) string {
	source := templateSource(tpl)
	if dir, err := Dir(); err == nil && within(lazypath.Canonical(dir), lazypath.Canonical(source)) {
		return dir
	}
	return source
}

// confinedRead reads a file of a template, refusing a symbolic link that
// leaves root.
//
// A link that stays inside the root is read as the file it points at, so a
// store assembled from links into itself keeps working; a link that leaves it
// is the one thing a template must not be able to do without being looked
// at.
func confinedRead(root, path string) ([]byte, error) {
	if err := confinedSource(root, path); err != nil {
		return nil, err
	}
	return os.ReadFile(path)
}

// confinedSource reports whether a file of a template may be read: the whole
// path is resolved, so a link anywhere on it, a folder as much as the file,
// must stay inside the root.
func confinedSource(root, path string) error {
	if _, err := os.Lstat(path); err != nil {
		return err
	}
	if root == "" || !within(lazypath.Canonical(root), lazypath.Canonical(path)) {
		return fmt.Errorf("%s is a symbolic link pointing outside the template; a template writes a link with the %s prefix instead", path, AttrSymlink)
	}
	return nil
}

// confinedTarget reports whether a rendered file may be written at target:
// it is refused when a link already on disk, written by an earlier file of
// the same template or by anyone else, would carry it outside root.
func confinedTarget(root, target string) error {
	if !within(lazypath.Canonical(root), lazypath.Canonical(target)) {
		return fmt.Errorf("refusing to write %s: a symbolic link carries it outside %s", target, root)
	}
	return nil
}
