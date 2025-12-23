package folderutil

import (
	"testing"

	"gitlab.com/dynamo.foss/projekt/pkg/lazypath"
)

func BenchmarkParseConfig(b *testing.B) {
	config := lazypath.Config{
		Folders: []lazypath.Folder{
			{
				Path:        "/tmp/test1",
				Prefix:      "test",
				IsWorkspace: false,
			},
			{
				Path:        "/tmp/test2",
				Prefix:      "test",
				IsWorkspace: false,
			},
			{
				Path:        "/tmp/test3",
				Prefix:      "test",
				IsWorkspace: false,
			},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := ParseConfig(config)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkAppendToParsedFolder(b *testing.B) {
	list := make([]ParsedFolder, 0, 10)
	for i := 0; i < 5; i++ {
		list = append(list, ParsedFolder{
			ShortName: "test-project" + string(rune(i)),
			Path:      "/tmp/project" + string(rune(i)),
			Workspace: "/tmp",
		})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = appendToParsedFolder(list, "test-", "/tmp/workspace", "newproject")
	}
}

func BenchmarkParseConfigWithWorkspace(b *testing.B) {
	tmpDir := b.TempDir()

	config := lazypath.Config{
		Folders: []lazypath.Folder{
			{
				Path:        tmpDir,
				Prefix:      "bench",
				IsWorkspace: true,
				RegexMatch:  ".*",
			},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := ParseConfig(config)
		if err != nil {
			b.Fatal(err)
		}
	}
}
