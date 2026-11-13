package folder

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"gitlab.com/dynamo-tools/projekt/pkg/folderutil"
)

const describeLongHelp = `Say what a project is, in one line.

A short name says where a project is, not what it is. The description is kept
in the configuration, shown by 'folder list', offered beside the name when
you press tab after 'pj', and drawn next to the candidates 'pj' asks you to
choose from.

  projekt folder describe billing "The invoicing API, Go, owned by payments"
  projekt folder describe billing              # print it
  projekt folder describe billing --clear      # take it away

A project found inside a workspace has no entry of its own and carries the
workspace's description: describe the workspace.

With --generate, the AI command you name in ai.command (or PROJEKT_AI) drafts
the description. It is sent the project's short name, a list of its paths and
the start of its README: never a secret, a git-ignored file or a big file.
Nothing is written until you answer yes (or pass --yes); --show-prompt prints
what would be sent and stops.`

func NewFolderDescribeCmd(out io.Writer) *cobra.Command {
	var clear, generate, yes, showPrompt bool

	cmd := &cobra.Command{
		Use:     "describe <short name> [description...]",
		Short:   "Set or print the one-line description of a project",
		Long:    describeLongHelp,
		Args:    cobra.MinimumNArgs(1),
		Aliases: []string{"desc"},
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			if len(args) == 0 {
				return compListShortNames(toComplete)
			}
			return nil, cobra.ShellCompDirectiveNoFileComp
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if generate {
				if clear || len(args) > 1 {
					return fmt.Errorf("--generate drafts the description itself: give no text and no --clear")
				}
				return folderutil.GenerateDescription(args[0], folderutil.GenerateOptions{
					ShowPrompt: showPrompt, Yes: yes, In: os.Stdin, Out: out, Err: cmd.ErrOrStderr(),
				})
			}
			if yes || showPrompt {
				return fmt.Errorf("--yes and --show-prompt go with --generate")
			}
			return folderutil.DescribeFolder(out, args[0], strings.Join(args[1:], " "), clear)
		},
	}

	cmd.Flags().BoolVar(&clear, "clear", false, "Remove the description")
	cmd.Flags().BoolVar(&generate, "generate", false, "Draft the description with your AI command")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "With --generate, keep the draft without asking")
	cmd.Flags().BoolVar(&showPrompt, "show-prompt", false, "With --generate, print what would be sent and stop")

	return cmd
}
