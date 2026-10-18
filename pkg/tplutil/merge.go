package tplutil

import (
	"bytes"
	"strings"
	"unicode/utf8"
)

// merge3 combines two sets of changes to the same text: what was edited by
// hand in the project (ours) and what the template now renders (theirs), both
// measured against what the template rendered last time (base).
//
// A region only one side changed takes that side; a region both changed the
// same way takes it once; a region they changed differently is a conflict,
// and the whole merge is reported unclean — nothing is written with markers
// in it, since a half-merged file is worse than a kept one. Binary content
// and very large files are not merged.
func merge3(base, ours, theirs []byte) ([]byte, bool) {
	if !mergeable(base) || !mergeable(ours) || !mergeable(theirs) {
		return nil, false
	}
	b, o, t := splitKeep(base), splitKeep(ours), splitKeep(theirs)
	if len(b) > diffLimit || len(o) > diffLimit || len(t) > diffLimit {
		return nil, false
	}

	inOurs := lcsPairs(b, o)
	inTheirs := lcsPairs(b, t)

	var out bytes.Buffer
	bi, oi, ti := 0, 0, 0
	for i := 0; i <= len(b); i++ {
		oj, oursHas := inOurs[i]
		tj, theirsHas := inTheirs[i]
		if i == len(b) {
			oj, tj, oursHas, theirsHas = len(o), len(t), true, true
		}
		if !oursHas || !theirsHas {
			// Not a line both sides kept: part of the region being decided.
			continue
		}

		region, ok := resolve(b[bi:i], o[oi:oj], t[ti:tj])
		if !ok {
			return nil, false
		}
		for _, line := range region {
			out.WriteString(line)
		}
		if i < len(b) {
			out.WriteString(b[i])
		}
		bi, oi, ti = i+1, oj+1, tj+1
	}
	return out.Bytes(), true
}

// resolve decides one region between two anchors.
func resolve(base, ours, theirs []string) ([]string, bool) {
	switch {
	case equalLines(ours, base):
		return theirs, true
	case equalLines(theirs, base):
		return ours, true
	case equalLines(ours, theirs):
		return ours, true
	default:
		return nil, false
	}
}

func equalLines(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// splitKeep cuts text into lines that keep their line ending, so that joining
// them back gives the text exactly, final newline or not.
func splitKeep(content []byte) []string {
	if len(content) == 0 {
		return nil
	}
	lines := strings.SplitAfter(string(content), "\n")
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

// lcsPairs matches the lines of a to the lines of b along their longest common
// subsequence, as a map from an index in a to the index in b it pairs with.
func lcsPairs(a, b []string) map[int]int {
	lengths := make([][]int, len(a)+1)
	for i := range lengths {
		lengths[i] = make([]int, len(b)+1)
	}
	for i := len(a) - 1; i >= 0; i-- {
		for j := len(b) - 1; j >= 0; j-- {
			if a[i] == b[j] {
				lengths[i][j] = lengths[i+1][j+1] + 1
				continue
			}
			lengths[i][j] = max(lengths[i+1][j], lengths[i][j+1])
		}
	}

	pairs := make(map[int]int)
	i, j := 0, 0
	for i < len(a) && j < len(b) {
		switch {
		case a[i] == b[j]:
			pairs[i] = j
			i++
			j++
		case lengths[i+1][j] >= lengths[i][j+1]:
			i++
		default:
			j++
		}
	}
	return pairs
}

// mergeable reports whether content is worth merging line by line: valid UTF-8
// with no NUL byte in it. Nothing is nil-safe about a binary merge.
func mergeable(content []byte) bool {
	if bytes.IndexByte(content, 0) >= 0 {
		return false
	}
	return utf8.Valid(content)
}
