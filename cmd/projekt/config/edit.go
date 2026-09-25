package config

import (
	"fmt"
	"io"
	"os"
	"os/exec"

	"github.com/spf13/cobra"

	"gitlab.com/dynamo-tools/projekt/pkg/cli"
	"gitlab.com/dynamo-tools/projekt/pkg/lazypath"
)

func NewConfigEditCmd(out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "edit",
		Aliases: []string{"e"},
		Short:   "Open the configuration file in your editor",
		Long: `Open the configuration file in $VISUAL, or $EDITOR, or vi.

The file is re-read once the editor exits, so a mistake is reported straight
away rather than on the next command.`,
		Args: cobra.NoArgs,
		// Editing is how a broken config gets fixed, so this has to run even
		// when the file cannot currently be read.
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error { return nil },
		RunE: func(cmd *cobra.Command, args []string) error {
			return runConfigEdit(out, cmd.InOrStdin(), cmd.ErrOrStderr())
		},
	}

	cli.SetColorAndStyles(cmd)
	return cmd
}

func runConfigEdit(out io.Writer, in io.Reader, errOut io.Writer) error {
	path := lazypath.ConfigFile()
	if path == "" {
		return fmt.Errorf("no configuration file to edit")
	}

	editor := cli.EditorCommand(os.Getenv)
	cli.Debug("Opening %s with %v", path, editor)

	args := append(editor[1:], path)
	cmd := exec.Command(editor[0], args...)
	// An editor is interactive: it needs the real terminal, not a buffer.
	cmd.Stdin = in
	cmd.Stdout = out
	cmd.Stderr = errOut

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("editor %s failed: %w", editor[0], err)
	}

	if err := lazypath.ReloadConfig(); err != nil {
		return fmt.Errorf("config is no longer readable after editing: %w", err)
	}

	config := lazypath.GetConfig()
	diags := config.Diagnose()

	var errorCount int
	for _, diag := range diags {
		if diag.Severity == lazypath.SeverityError {
			errorCount++
		}
		if _, err := fmt.Fprintf(errOut, "[%s] %s\n", diag.Severity, diag.Message); err != nil {
			return err
		}
	}

	if errorCount > 0 {
		return fmt.Errorf("configuration has %d error(s) after editing", errorCount)
	}

	return nil
}
