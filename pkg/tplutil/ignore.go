package tplutil

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// IgnoreFile lists what a folder template does *not* write.
//
// It is rendered like any other file first, so the decision can be made from
// the values — `{{ if not .Values.ci }}.github/{{ end }}` leaves the workflow
// out — and then read as a set of gitignore style patterns matched against the
// paths the template would have created.
const IgnoreFile = ".ignore"

// ignoreRule is one line of the ignore file.
type ignoreRule struct {
	// segments is the pattern, split on "/".
	segments []string
	// negate is a "!" line, which brings back what an earlier line dropped.
	negate bool
	// dirOnly is a pattern written with a trailing "/".
	dirOnly bool
	// anchored is a pattern with a "/" in it, matched from the root of the
	// output rather than against any file name.
	anchored bool
}

// IgnoreSet decides which paths a folder template skips.
type IgnoreSet struct {
	rules []ignoreRule
}

// LoadIgnore reads and renders a folder template's ignore file.
//
// A template without one ignores nothing, which is the normal case.
func (e *engine) loadIgnore(o RenderOptions, data map[string]any) (*IgnoreSet, error) {
	if !o.Template.IsDir() {
		return &IgnoreSet{}, nil
	}
	file := filepath.Join(o.Template.Path, IgnoreFile)

	raw, err := os.ReadFile(file)
	if err != nil {
		if os.IsNotExist(err) {
			return &IgnoreSet{}, nil
		}
		return nil, fmt.Errorf("cannot read %s: %w", file, err)
	}

	rendered, err := e.execute(o.Template.Name+"/"+IgnoreFile, string(raw), data)
	if err != nil {
		return nil, err
	}
	return ParseIgnore(string(rendered)), nil
}

// ParseIgnore reads the patterns of an already rendered ignore file.
func ParseIgnore(text string) *IgnoreSet {
	set := &IgnoreSet{}
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		rule := ignoreRule{}
		if strings.HasPrefix(line, "!") {
			rule.negate = true
			line = strings.TrimSpace(strings.TrimPrefix(line, "!"))
		}
		if strings.HasSuffix(line, "/") {
			rule.dirOnly = true
			line = strings.TrimSuffix(line, "/")
		}
		if line == "" {
			continue
		}
		// A pattern that says where it is, is matched from the root; a bare
		// name matches at any depth, the way .gitignore reads.
		rule.anchored = strings.Contains(line, "/")
		line = strings.TrimPrefix(line, "/")
		rule.segments = strings.Split(line, "/")
		set.rules = append(set.rules, rule)
	}
	return set
}

// Empty reports whether the set would never skip anything.
func (s *IgnoreSet) Empty() bool {
	return s == nil || len(s.rules) == 0
}

// Match reports whether a rendered path is left out. The path is relative to
// the destination folder and uses "/" whatever the platform is.
//
// The last rule that matches decides, so a "!" line after a broad pattern
// brings a file back.
func (s *IgnoreSet) Match(rel string, isDir bool) bool {
	if s.Empty() {
		return false
	}
	rel = filepath.ToSlash(rel)
	ignored := false
	for _, rule := range s.rules {
		if rule.dirOnly && !isDir {
			continue
		}
		if rule.match(rel) {
			ignored = !rule.negate
		}
	}
	return ignored
}

// match reports whether one rule covers a path.
func (r ignoreRule) match(rel string) bool {
	segments := strings.Split(rel, "/")
	if r.anchored {
		return matchSegments(r.segments, segments)
	}
	// A bare name matches the file itself and anything under a folder of that
	// name, at any depth.
	for i := range segments {
		if matchSegments(r.segments, segments[i:]) {
			return true
		}
	}
	return false
}

// matchSegments matches a split pattern against a split path, with "**"
// standing for any number of segments, including none.
//
// A pattern that runs out while the path has segments left still matches: a
// folder that is ignored takes its contents with it.
func matchSegments(pattern, name []string) bool {
	switch {
	case len(pattern) == 0:
		return true
	case pattern[0] == "**":
		for i := 0; i <= len(name); i++ {
			if matchSegments(pattern[1:], name[i:]) {
				return true
			}
		}
		return false
	case len(name) == 0:
		return false
	}

	ok, err := path.Match(pattern[0], name[0])
	if err != nil || !ok {
		return false
	}
	return matchSegments(pattern[1:], name[1:])
}
