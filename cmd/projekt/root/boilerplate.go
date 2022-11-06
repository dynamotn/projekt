package root

import (
	"io"

	"github.com/spf13/cobra"

	b "gitlab.com/dynamo.foss/projekt/cmd/b/root"
)

func NewBoilerplateCmd(out io.Writer) *cobra.Command {
	return b.NewProjektBoilerplateCmd(out)
}
