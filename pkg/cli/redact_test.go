package cli

import "testing"

func TestRedactURL(t *testing.T) {
	tests := map[string]string{
		"https://user:s3cret@github.com/me/repo.git": "https://xxxxx@github.com/me/repo.git",
		"https://ghp_token@github.com/me/repo.git":   "https://xxxxx@github.com/me/repo.git",
		"https://github.com/me/repo.git":             "https://github.com/me/repo.git",
		"git@github.com:me/repo.git":                 "git@github.com:me/repo.git",
		"ssh://git@github.com:2222/me/repo.git":      "ssh://xxxxx@github.com:2222/me/repo.git",
		"https://tok%zz@host/repo":                   "https://xxxxx@host/repo",
		"https://a@b@host/repo":                      "https://xxxxx@host/repo",
		"https://a/b@host/repo":                      "https://xxxxx@host/repo",
		"https://user:p@ss@host/repo?x=1#f":          "https://xxxxx@host/repo?x=1#f",
		"https://user:pass@host/repo?token=a@b":      "https://xxxxx@host/repo?token=a@b",
		"https://user:pass@host/repo#frag@x":         "https://xxxxx@host/repo#frag@x",
		"https://host/path/@scope/pkg":               "https://xxxxx@scope/pkg",
		"":                                           "",
	}
	for raw, want := range tests {
		if got := RedactURL(raw); got != want {
			t.Errorf("RedactURL(%q) = %q, want %q", raw, got, want)
		}
	}
}
