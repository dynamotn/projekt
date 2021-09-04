package main

import (
	"log"

	"github.com/spf13/cobra/doc"

	projekt "gitlab.com/dynamo.foss/projekt/pkg/projekt/cli"
	pj "gitlab.com/dynamo.foss/projekt/pkg/pj/cli"
	t "gitlab.com/dynamo.foss/projekt/pkg/t/cli"
	b "gitlab.com/dynamo.foss/projekt/pkg/b/cli"
)

func main() {
	var err error
	err = doc.GenMarkdownTree(projekt.RootCmd, "doc")
	if err != nil {
		log.Fatal(err)
	}
	err = doc.GenMarkdownTree(pj.RootCmd, "doc")
	if err != nil {
		log.Fatal(err)
	}
	err = doc.GenMarkdownTree(t.RootCmd, "doc")
	if err != nil {
		log.Fatal(err)
	}
	err = doc.GenMarkdownTree(b.RootCmd, "doc")
	if err != nil {
		log.Fatal(err)
	}
}
