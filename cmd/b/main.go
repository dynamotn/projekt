package main

import (
	"gitlab.com/dynamo-tools/projekt/cmd/b/root"
	"gitlab.com/dynamo-tools/projekt/pkg/cli"
)

func main() {
	cli.InitLogging()
	root.Execute()
}
