package main

import (
	"gitlab.com/dynamo-tools/projekt/cmd/t/root"
	"gitlab.com/dynamo-tools/projekt/pkg/cli"
)

func main() {
	cli.InitLogging()
	root.Execute()
}
