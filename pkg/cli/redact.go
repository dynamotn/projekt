package cli

import "strings"

// RedactURL hides the user information of a URL, for a message: a token
// pasted into an https URL belongs in the configuration, not in a terminal
// or a CI log.
//
// It does not parse the URL: a parser that disagrees with git about where
// the user information ends would let a token through. Everything between
// `://` and the last `@` before the query or the fragment is hidden, user
// name included, because a token is usually written there on its own. Hiding
// too much of an odd URL is fine; showing too little of one is not. The scp
// form `git@host:path` is kept: `git` is a login name every server
// publishes, not a secret.
func RedactURL(raw string) string {
	scheme, rest, found := strings.Cut(raw, "://")
	if !found {
		return raw
	}
	head, tail := rest, ""
	if end := strings.IndexAny(rest, "?#"); end >= 0 {
		head, tail = rest[:end], rest[end:]
	}
	at := strings.LastIndex(head, "@")
	if at < 0 {
		return raw
	}
	return scheme + "://xxxxx@" + head[at+1:] + tail
}
