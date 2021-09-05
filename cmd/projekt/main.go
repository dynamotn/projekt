package main

import (
	"gitlab.com/dynamo.foss/projekt/cmd/projekt/root"
	"gitlab.com/dynamo.foss/projekt/pkg/cli"
)

func main() {
	cli.InitLogging()
	root.Execute()
}
