package root

import (
	"io"

	"github.com/spf13/cobra"

	t "gitlab.com/dynamo.foss/projekt/cmd/t/root"
)

func NewTemplateCmd(out io.Writer) *cobra.Command {
	return t.NewProjektTemplateCmd(out)
}
