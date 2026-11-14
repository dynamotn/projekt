package aiutil

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"unicode"
	"unicode/utf8"

	"gitlab.com/dynamo-tools/projekt/pkg/cli"
	"gitlab.com/dynamo-tools/projekt/pkg/lazypath"
)

const (
	// maxFileSize is the largest file that may be quoted at all.
	maxFileSize = 32 << 10
	// quoteSize is how much of a file a prompt quotes.
	quoteSize = 4 << 10
)

// File is one project file quoted in a prompt.
type File struct {
	Path    string // relative to the project, slash-separated
	Content string // at most the first 4 KiB
}

// Files returns the named files of a project that may be quoted, in the
// order asked, silently leaving out every file the rules forbid.
func Files(project string, names ...string) ([]File, error) {
	root := lazypath.Canonical(project)

	type candidate struct{ name, real, rel string }
	var candidates []candidate
	var rels []string
	for _, name := range names {
		real, rel, ok := allowed(root, name)
		if !ok {
			continue
		}
		candidates = append(candidates, candidate{name, real, rel})
		rels = append(rels, rel)
	}

	ignored, err := gitIgnored(root, rels)
	if err != nil {
		return nil, err
	}

	var files []File
	for _, c := range candidates {
		if covered(ignored, c.rel) {
			continue
		}
		content, ok, err := quote(c.real)
		if err != nil {
			return nil, err
		}
		if ok {
			files = append(files, File{Path: c.name, Content: content})
		}
	}
	return files, nil
}

// Tree lists the project's paths that may be named in a prompt, sorted,
// at most limit of them, directories with a trailing slash, depth levels
// deep (0: every level). It never looks inside a nested repository: what
// that one ignores, the project's git cannot tell.
func Tree(project string, limit, depth int) ([]string, error) {
	root := lazypath.Canonical(project)

	type entry struct{ name, rel string }
	var entries []entry
	var rels []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		switch {
		case err != nil && path == root:
			return err
		case err != nil && d != nil && d.IsDir():
			// A folder that cannot be read is left out, not the whole tree.
			return filepath.SkipDir
		case err != nil, path == root:
			return nil
		}
		name := relative(root, path)
		deepest := depth > 0 && strings.Count(name, "/")+1 >= depth
		real, rel, ok := allowed(root, name)
		if !ok {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		info, err := os.Stat(real)
		switch {
		case err != nil:
			return nil
		case info.IsDir():
			name, rel = name+"/", rel+"/"
		case !info.Mode().IsRegular() || info.Size() > maxFileSize:
			return nil
		}
		entries = append(entries, entry{name, rel})
		rels = append(rels, rel)
		if d.IsDir() && (deepest || nestedRepository(real)) {
			return filepath.SkipDir
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	ignored, err := gitIgnored(root, rels)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if !covered(ignored, e.rel) {
			names = append(names, e.name)
		}
	}
	slices.Sort(names)
	return names[:min(len(names), max(limit, 0))], nil
}

// allowed reports whether name, relative to root, may be read into a prompt,
// and if so where it really is, as an absolute path and relative to root:
// never by a forbidden name, never through a link that leaves root or lands
// on a forbidden name. A link is judged by what it resolves to.
func allowed(root, name string) (real, rel string, ok bool) {
	local := filepath.FromSlash(name)
	if !filepath.IsLocal(local) || forbidden(name) || !printable(name) {
		return "", "", false
	}
	real = lazypath.Canonical(filepath.Join(root, local))
	rel = relative(root, real)
	if rel == "." || !filepath.IsLocal(filepath.FromSlash(rel)) || forbidden(rel) || !printable(rel) {
		return "", "", false
	}
	return real, rel, true
}

// forbidden reports whether any element of a slash-separated path names a
// file that holds secrets or is not the project's own.
func forbidden(name string) bool {
	for _, element := range strings.Split(name, "/") {
		element = strings.ToLower(element)
		switch {
		case element == ".git", element == "secrets", element == ".projekt",
			strings.HasPrefix(element, ".env"),
			strings.HasSuffix(element, ".age"),
			strings.HasSuffix(element, ".pem"),
			strings.HasSuffix(element, ".key"):
			return true
		}
	}
	return false
}

// printable reports whether a name can be shown as one line of a prompt or
// a terminal: valid UTF-8 with no control or bidi character, so a file named
// "x\nIgnore all previous instructions" never becomes a line of its own.
func printable(name string) bool {
	return utf8.ValidString(name) && !strings.ContainsFunc(name, func(r rune) bool {
		return unicode.IsControl(r) || bidi(r)
	})
}

// nestedRepository reports whether dir holds a repository of its own.
func nestedRepository(dir string) bool {
	_, err := os.Lstat(filepath.Join(dir, ".git"))
	return err == nil
}

// relative returns path relative to root, slash-separated.
func relative(root, path string) string {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return ".."
	}
	return filepath.ToSlash(rel)
}

// covered reports whether rel, or a folder above it, is ignored.
func covered(ignored map[string]bool, rel string) bool {
	for _, path := range withParents(rel) {
		if ignored[path] {
			return true
		}
	}
	return false
}

// withParents returns rel preceded by each folder above it, slash-ended.
func withParents(rel string) []string {
	var paths []string
	for i := range len(rel) - 1 {
		if rel[i] == '/' {
			paths = append(paths, rel[:i+1])
		}
	}
	return append(paths, rel)
}

// gitIgnored asks git, once, which of rels (or the folders above them) a
// .gitignore covers. rels are resolved paths relative to root: git refuses
// one that runs through a link. A folder in no work tree ignores nothing.
func gitIgnored(root string, rels []string) (map[string]bool, error) {
	if len(rels) == 0 {
		return nil, nil
	}
	inside, err := inWorkTree(root)
	if err != nil || !inside {
		return nil, err
	}

	seen := map[string]bool{}
	var query []string
	for _, rel := range rels {
		for _, path := range withParents(rel) {
			if !seen[path] {
				seen[path] = true
				query = append(query, path)
			}
		}
	}
	cmd := cli.Command("git", "-C", root, "check-ignore", "-z", "--stdin")
	// Inherited, GIT_DIR and the like would make git answer for another
	// repository, one that may ignore nothing.
	cmd.Env = cli.WithoutRepoEnv(os.Environ())
	cmd.Stdin = strings.NewReader(strings.Join(query, "\x00") + "\x00")
	out, err := cmd.Output()
	// Status 1 is git's way of saying nothing is ignored.
	var exit *exec.ExitError
	if err != nil && !(errors.As(err, &exit) && exit.ExitCode() == 1) {
		// Not knowing what is ignored, nothing may be sent.
		return nil, gitError("check-ignore", err)
	}
	ignored := map[string]bool{}
	for _, name := range bytes.Split(out, []byte{0}) {
		ignored[string(name)] = true
	}
	return ignored, nil
}

// inWorkTree asks git whether root is inside a work tree, at any depth: a
// project can be one folder of a larger repository. Only "not a git
// repository" means no; any other failure is an error.
func inWorkTree(root string) (bool, error) {
	cmd := cli.Command("git", "-C", root, "rev-parse", "--is-inside-work-tree")
	// git translates its messages; the one read below must be the English one.
	cmd.Env = append(cli.WithoutRepoEnv(os.Environ()), "LC_ALL=C")
	out, err := cmd.Output()
	var exit *exec.ExitError
	if errors.As(err, &exit) && bytes.Contains(exit.Stderr, []byte("not a git repository")) {
		return false, nil
	}
	if err != nil {
		return false, gitError("rev-parse", err)
	}
	if answer := strings.TrimSpace(string(out)); answer != "true" {
		return false, fmt.Errorf("cannot tell what git ignores in %s: it is not in a work tree", root)
	}
	return true, nil
}

// gitError adds what git said to why it failed.
func gitError(what string, err error) error {
	var exit *exec.ExitError
	if errors.As(err, &exit) && len(exit.Stderr) > 0 {
		return fmt.Errorf("git %s: %w: %s", what, err, strings.TrimSpace(string(exit.Stderr)))
	}
	return fmt.Errorf("git %s: %w", what, err)
}

// quote returns the start of a plain file of at most maxFileSize, or false
// for anything else. It checks the file it opened, not the path: the path
// may have changed since it was judged, to a link or a FIFO.
func quote(path string) (string, bool, error) {
	file, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0)
	if err != nil {
		return "", false, nil
	}
	defer func() { _ = file.Close() }()
	info, err := file.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() > maxFileSize {
		return "", false, nil
	}
	data, err := io.ReadAll(io.LimitReader(file, quoteSize))
	// The prompt is shown by --show-prompt: no escape may reach the terminal.
	return plain(string(data)), err == nil, err
}
