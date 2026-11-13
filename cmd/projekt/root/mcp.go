package root

import (
	"io"

	"github.com/spf13/cobra"

	"gitlab.com/dynamo-tools/projekt/internal/mcp"
)

const mcpLongHelp = `Serve what projekt knows to an AI agent, over MCP on stdin and stdout.

The tools are project_list, project_show, project_status, project_recent and
project_current: where each project is, its tags, description and note, its
working trees and the git state of its repositories. Without --write every
tool is read-only. No tool ever runs a command, moves or removes a project,
or changes what is trusted.

With --write the server also offers note_add (append a dated line to a
project's note), describe_set (set a project's description) and worktree_add
(create a working tree on a branch). Without the flag they are neither
listed nor callable.

Register it as a stdio server with the command 'projekt' and the argument
'mcp':

  claude mcp add projekt -- projekt mcp`

func NewMCPCmd(out io.Writer) *cobra.Command {
	var write bool
	cmd := &cobra.Command{
		Use:   "mcp",
		Short: "Serve what projekt knows to AI agents over MCP",
		Long:  mcpLongHelp,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return mcp.NewServer(write).Serve(cmd.InOrStdin(), out)
		},
	}
	cmd.Flags().BoolVar(&write, "write", false,
		"Also offer the tools that change something: note_add, describe_set, worktree_add")
	return cmd
}
