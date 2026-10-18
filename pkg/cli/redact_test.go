package cli

import "testing"

func TestRedactURL(t *testing.T) {
	tests := map[string]string{
		"https://user:s3cret@github.com/me/repo.git": "https://user:xxxxx@github.com/me/repo.git",
		"https://github.com/me/repo.git":             "https://github.com/me/repo.git",
		"https://token@github.com/me/repo.git":       "https://token@github.com/me/repo.git",
		"git@github.com:me/repo.git":                 "git@github.com:me/repo.git",
		"ssh://git@github.com:2222/me/repo.git":      "ssh://git@github.com:2222/me/repo.git",
		"":                                           "",
	}
	for raw, want := range tests {
		if got := RedactURL(raw); got != want {
			t.Errorf("RedactURL(%q) = %q, want %q", raw, got, want)
		}
	}
}
