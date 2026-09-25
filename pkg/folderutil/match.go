package folderutil

import (
	"sort"
	"strings"

	"gitlab.com/dynamo-tools/projekt/pkg/lazypath"
)

// How well a query matched a project, from the one that needs no thought to
// the one that needed the most.
type matchKind int

const (
	matchNone matchKind = iota
	// matchSubsequence: every letter of the query appears, in order —
	// "bak" in "bac[k]end-api".
	matchSubsequence
	// matchSubstring: the query appears as it is, somewhere.
	matchSubstring
	// matchPrefix: the name starts with the query.
	matchPrefix
	// matchInsensitive: the whole name, but for its case.
	matchInsensitive
	// matchExact: what was typed.
	matchExact
)

// Match is one project a query could have meant.
type Match struct {
	Folder ParsedFolder
	kind   matchKind
	score  float64
}

// Exact reports whether the query was the project's name.
func (m Match) Exact() bool {
	return m.kind == matchExact
}

// MatchFolders returns every project a query could have meant, best first.
//
// An exact name always wins, so typing a whole name can never be surprised by
// a cleverer interpretation of it. Below that, a closer kind of match beats a
// looser one, and within one kind the project you have been in most recently
// and most often comes first: with two candidates equally spelled, the one you
// actually use is the one you meant.
func MatchFolders(folders []ParsedFolder, query string) []Match {
	query = strings.TrimSpace(query)
	if query == "" {
		return matchesOf(folders)
	}

	scores := lazypath.Scores()

	matches := make([]Match, 0, len(folders))
	for _, folder := range folders {
		kind := classify(folder.ShortName, query)
		if kind == matchNone {
			continue
		}
		matches = append(matches, Match{Folder: folder, kind: kind, score: scores[folder.ShortName]})
	}

	sortMatches(matches)
	return matches
}

// matchesOf turns every folder into a candidate, for an empty query.
func matchesOf(folders []ParsedFolder) []Match {
	scores := lazypath.Scores()

	matches := make([]Match, 0, len(folders))
	for _, folder := range folders {
		matches = append(matches, Match{Folder: folder, kind: matchSubsequence, score: scores[folder.ShortName]})
	}

	sortMatches(matches)
	return matches
}

// sortMatches orders candidates: closer kind of match, then the one used most,
// then the shorter name, then by name so that two runs agree.
//
// Length matters because the same kind of match on a shorter name is a tighter
// one: "backend" is all of "backend-api" but only half of
// "backend-api-docs-archive", and it is the first that was meant.
func sortMatches(matches []Match) {
	sort.SliceStable(matches, func(i, j int) bool {
		if matches[i].kind != matches[j].kind {
			return matches[i].kind > matches[j].kind
		}
		if matches[i].score != matches[j].score {
			return matches[i].score > matches[j].score
		}
		left, right := matches[i].Folder.ShortName, matches[j].Folder.ShortName
		if len(left) != len(right) {
			return len(left) < len(right)
		}
		return left < right
	})
}

// classify says how a name matched a query.
func classify(name, query string) matchKind {
	if name == query {
		return matchExact
	}

	lowerName := strings.ToLower(name)
	lowerQuery := strings.ToLower(query)

	switch {
	case lowerName == lowerQuery:
		return matchInsensitive
	case strings.HasPrefix(lowerName, lowerQuery):
		return matchPrefix
	case strings.Contains(lowerName, lowerQuery):
		return matchSubstring
	case isSubsequence(lowerName, lowerQuery):
		return matchSubsequence
	default:
		return matchNone
	}
}

// isSubsequence reports whether every letter of the query appears in the name,
// in order. It is what makes "bak" reach "backend-api", and it is the loosest
// rule here: anything looser matches everything.
func isSubsequence(name, query string) bool {
	// Compared rune by rune: indexing a string gives bytes, and a name with a
	// letter outside ASCII in it would match nonsense.
	wanted := []rune(query)
	if len(wanted) == 0 {
		return true
	}

	next := 0
	for _, letter := range name {
		if wanted[next] == letter {
			next++
			if next == len(wanted) {
				return true
			}
		}
	}
	return false
}
