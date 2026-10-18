package cli

import (
	"net/url"
	"regexp"
)

// userinfoPattern finds `scheme://anything@` in a string a URL parser could
// not read, so that a token still does not reach a log.
var userinfoPattern = regexp.MustCompile(`^([a-zA-Z][a-zA-Z0-9+.-]*://)[^/@]*@`)

// RedactURL hides the user information of a URL, for a message: a token
// pasted into an https URL belongs in the configuration, not in a terminal
// or a CI log. The user name goes too, because a token is usually written
// there on its own. The scp form `git@host:path` is kept: `git` is a login
// name every server publishes, not a secret.
func RedactURL(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil {
		return userinfoPattern.ReplaceAllString(raw, "${1}xxxxx@")
	}
	if parsed.Scheme == "" || parsed.User == nil {
		return raw
	}
	parsed.User = url.User("xxxxx")
	return parsed.String()
}
