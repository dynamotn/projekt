package cli

import "net/url"

// RedactURL hides the user information of a URL, for a message: a token
// pasted into an https URL belongs in the configuration, not in a terminal
// or a CI log. Anything that is not a URL with a scheme comes back as it is.
func RedactURL(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Scheme == "" || parsed.User == nil {
		return raw
	}
	if _, hasPassword := parsed.User.Password(); !hasPassword {
		return raw
	}
	parsed.User = url.UserPassword(parsed.User.Username(), "xxxxx")
	return parsed.String()
}
