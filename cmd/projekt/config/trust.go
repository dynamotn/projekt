package config

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"gitlab.com/dynamo-tools/projekt/pkg/cli"
	"gitlab.com/dynamo-tools/projekt/pkg/hookutil"
	"gitlab.com/dynamo-tools/projekt/pkg/lazypath"
)

func NewConfigTrustCmd(out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "trust [file...]",
		Short: "Let the hooks of an included configuration file run",
		Long: `Let the hooks of an included configuration file run, as it is now.

Your own configuration file is yours. One it includes from a repository — a
team's shared.yaml, a dotfiles checkout — can change under you, so the enter
and leave hooks it declares are handed to your shell only once you have looked
at that version of the file and trusted it. On a terminal, 'pj' asks the first
time; this trusts beforehand, for a shell started by a script.

  projekt config trust                      # every included file
  projekt config trust ~/dotfiles/team.yaml

Trust covers the exact content: a later change to the file asks again.`,
		Args: cobra.ArbitraryArgs,
		ValidArgsFunction: func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
			return lazypath.IncludedFiles(), cobra.ShellCompDirectiveNoFileComp
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			files := args
			if len(files) == 0 {
				files = lazypath.IncludedFiles()
			}
			if len(files) == 0 {
				_, err := fmt.Fprintln(out, "No included configuration file to trust")
				return err
			}
			for _, file := range files {
				if err := hookutil.TrustSource(file); err != nil {
					return fmt.Errorf("trust %s: %w", file, err)
				}
				if _, err := fmt.Fprintf(out, "Trusted %s\n", file); err != nil {
					return err
				}
			}
			return nil
		},
	}

	cli.SetColorAndStyles(cmd)
	return cmd
}
