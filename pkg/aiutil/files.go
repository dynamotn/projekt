package aiutil

import (
	"bytes"
	"errors"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

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

	type candidate struct{ name, real string }
	var candidates []candidate
	var asked []string
	for _, name := range names {
		real, info, ok := allowed(root, name)
		if !ok || info.IsDir() {
			continue
		}
		candidates = append(candidates, candidate{name, real})
		asked = append(asked, name, relative(root, real))
	}

	ignored, err := gitIgnored(root, asked)
	if err != nil {
		return nil, err
	}

	var files []File
	for _, c := range candidates {
		if ignored[c.name] || ignored[relative(root, c.real)] {
			continue
		}
		content, err := quote(c.real)
		if err != nil {
			return nil, err
		}
		files = append(files, File{Path: c.name, Content: content})
	}
	return files, nil
}

// Tree lists the project's paths that may be named in a prompt, sorted,
// at most limit of them, directories with a trailing slash.
func Tree(project string, limit int) ([]string, error) {
	root := lazypath.Canonical(project)

	var entries []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || path == root {
			return err
		}
		name := relative(root, path)
		_, info, ok := allowed(root, name)
		switch {
		case !ok && d.IsDir():
			return filepath.SkipDir
		case !ok:
			return nil
		case info.IsDir():
			name += "/"
		}
		entries = append(entries, name)
		return nil
	})
	if err != nil {
		return nil, err
	}

	ignored, err := gitIgnored(root, entries)
	if err != nil {
		return nil, err
	}
	entries = slices.DeleteFunc(entries, func(entry string) bool {
		// Under an ignored folder counts as ignored, whatever git says of
		// the path itself.
		for i := range entry {
			if entry[i] == '/' && ignored[entry[:i+1]] {
				return true
			}
		}
		return ignored[entry]
	})

	slices.Sort(entries)
	return entries[:min(len(entries), max(limit, 0))], nil
}

// allowed reports whether name, relative to root, may be read into a prompt,
// and if so where it really is: never by a forbidden name, never through a
// link that leaves root or lands on a forbidden name, never a file over
// maxFileSize, and only a plain file or folder.
func allowed(root, name string) (string, fs.FileInfo, bool) {
	local := filepath.FromSlash(name)
	if !filepath.IsLocal(local) || forbidden(name) {
		return "", nil, false
	}
	real := lazypath.Canonical(filepath.Join(root, local))
	inside, err := filepath.Rel(root, real)
	if err != nil || !filepath.IsLocal(inside) || forbidden(filepath.ToSlash(inside)) {
		return "", nil, false
	}
	info, err := os.Stat(real)
	if err != nil || (!info.IsDir() && (!info.Mode().IsRegular() || info.Size() > maxFileSize)) {
		return "", nil, false
	}
	return real, info, true
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

// relative returns path relative to root, slash-separated.
func relative(root, path string) string {
	rel, _ := filepath.Rel(root, path)
	return filepath.ToSlash(rel)
}

// gitIgnored asks git, once, which of names a .gitignore covers. A folder
// that is not a repository ignores nothing.
func gitIgnored(root string, names []string) (map[string]bool, error) {
	if _, err := os.Lstat(filepath.Join(root, ".git")); err != nil || len(names) == 0 {
		return nil, nil
	}
	cmd := cli.Command("git", "-C", root, "check-ignore", "-z", "--stdin")
	cmd.Stdin = strings.NewReader(strings.Join(names, "\x00") + "\x00")
	out, err := cmd.Output()
	// Status 1 is git's way of saying nothing is ignored.
	var exit *exec.ExitError
	if err != nil && !(errors.As(err, &exit) && exit.ExitCode() == 1) {
		// Not knowing what is ignored, nothing may be sent.
		return nil, err
	}
	ignored := map[string]bool{}
	for _, name := range bytes.Split(out, []byte{0}) {
		ignored[string(name)] = true
	}
	return ignored, nil
}

// quote returns the start of a file.
func quote(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() { _ = file.Close() }()
	data, err := io.ReadAll(io.LimitReader(file, quoteSize))
	return string(data), err
}
