package main

import (
	"os"

	"github.com/spf13/cobra/doc"

	b "gitlab.com/dynamo.foss/projekt/cmd/b/root"
	projekt "gitlab.com/dynamo.foss/projekt/cmd/projekt/root"
	t "gitlab.com/dynamo.foss/projekt/cmd/t/root"
	"gitlab.com/dynamo.foss/projekt/pkg/cli"
)

func main() {
	var err error
	err = doc.GenMarkdownTree(projekt.NewRootCmd(os.Stdout), "doc")
	if err != nil {
		cli.Error("Failed to generate document", err)
	}
	err = doc.GenMarkdownTree(t.NewRootCmd(os.Stdout), "doc")
	if err != nil {
		cli.Error("Failed to generate document", err)
	}
	err = doc.GenMarkdownTree(b.NewRootCmd(os.Stdout), "doc")
	if err != nil {
		cli.Error("Failed to generate document", err)
	}
}
