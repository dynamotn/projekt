package lazypath

import (
	"testing"
)

func BenchmarkCheckFolderExist(b *testing.B) {
	c = Config{
		Folders: []Folder{
			{Path: "/tmp/test1"},
			{Path: "/tmp/test2"},
			{Path: "/tmp/test3"},
			{Path: "/tmp/test4"},
			{Path: "/tmp/test5"},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = CheckFolderExist("/tmp/test3")
	}
}

func BenchmarkGetRegexMatch(b *testing.B) {
	folder := Folder{
		Path:        "/tmp/test",
		IsWorkspace: true,
		RegexMatch:  "^project-.*",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = folder.GetRegexMatch()
	}
}

func BenchmarkValidate(b *testing.B) {
	config := Config{
		Folders: []Folder{
			{Path: "/tmp/test1"},
			{Path: "/tmp/test2"},
			{Path: "/tmp/test3"},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = config.Validate()
	}
}
